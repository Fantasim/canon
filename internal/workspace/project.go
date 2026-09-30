package workspace

import (
	"context"
	"errors"
	"slices"
	"sync"
	"time"

	"github.com/fantasim/canonlang/internal/build"
)

// Cause is why a project has a new snapshot (API.md §12).
type Cause string

// Event is a snapshot published, why, and the files whose content changed, in byte order: an
// edit's names the files it wrote, not the other names they are read by, which api.Watch finds by
// comparing snapshots (log-2026-09-29 M4 P14-r4).
type Event struct {
	Snapshot *Snapshot
	Cause    Cause
	Files    []string
}

// Project holds the current snapshot, the revisions produced, the overlays and the writer (S7).
type Project struct {
	tmpl    *build.Project // the opened build, pointed at each snapshot's file system
	now     func() time.Time
	closing chan struct{} // closed by Close (O6)
	write   chan struct{} // the one writer's lock (S9)

	refreshMu sync.Mutex // one refresh at a time; a writer's refreshes and writes among them

	mu      sync.Mutex
	cur     *Snapshot
	seq     uint64 // snapshots published, the number of the current one (S10)
	closed  bool
	writing bool // a writer runs: readers take the current snapshot as it is (S9)
	begun   int  // refreshes started, each numbered
	done    int  // the last refresh finished: a call that took its ticket before it need not refresh (S1)
	subs    []*subscriber
	calls   map[*call]bool // the shared computations running, which Close cancels
	hist    history
	watches watchSet // the watches running and the watcher they share (API.md W16)
}

type subscriber struct {
	fn func(Event)
}

// New is a project over b, the build Open returned; its first snapshot has read nothing yet.
// Its snapshots' builds share one build.Cache, the NFR-02 memo.
func New(b *build.Project) *Project {
	p := &Project{tmpl: b.WithCache(build.NewCache()), now: time.Now, closing: make(chan struct{}), write: make(chan struct{}, 1), calls: map[*call]bool{}}
	fs := newSnapFS(b.FS(), nil, p.now)
	fs.root = b.Dir()
	p.cur = p.snapshot(fs)
	return p
}

// snapshot is a snapshot over fs.
func (p *Project) snapshot(fs *snapFS) *Snapshot {
	return &Snapshot{p: p, fs: fs, b: p.tmpl.Over(fs), calls: map[string]*call{}}
}

// Close stops every shared computation and subscriber; every later Read or Write is ErrClosed.
// It is idempotent (API.md O6).
func (p *Project) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return
	}
	p.closed, p.subs = true, nil
	close(p.closing)
	//canon:unordered each running computation is cancelled on its own
	for c := range p.calls {
		c.cancel()
	}
}

// closedOr is ErrClosed for a computation Close cancelled, else err (O6).
func (p *Project) closedOr(err error) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed && errors.Is(err, context.Canceled) {
		return ErrClosed
	}
	return err
}

// running notes c started or done; one started after Close is cancelled at once (O6).
func (p *Project) running(c *call, on bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	switch {
	case !on:
		delete(p.calls, c)
	case p.closed:
		c.cancel()
	default:
		p.calls[c] = true
	}
}

// Read is the snapshot a call runs against from start to end: the current one, refreshed from
// the disk first unless a writer is running or a watch keeps it fresh (API.md S1, S9, S11).
func (p *Project) Read(ctx context.Context) (*Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	p.mu.Lock()
	ticket := p.begun
	p.mu.Unlock()
	p.refreshMu.Lock()
	defer p.refreshMu.Unlock()
	s, writing, err := p.current()
	if err != nil || writing {
		return s, err
	}
	p.mu.Lock()
	fresh := p.done > ticket
	p.mu.Unlock()
	if fresh {
		return s, nil
	}
	return p.refresh(ctx, s, CauseExternal)
}

// current is the current snapshot and whether a reader must take it as it is, a writer running
// or a watch keeping it fresh (S1, S9), or ErrClosed.
func (p *Project) current() (*Snapshot, bool, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil, false, ErrClosed
	}
	return p.cur, p.writing || p.watches.running(), nil
}

// refresh publishes s's successor when the disk changed under it, and returns the snapshot
// now current; the caller holds refreshMu.
func (p *Project) refresh(ctx context.Context, s *Snapshot, cause Cause) (*Snapshot, error) {
	next, changed, err := p.recheck(ctx, s.fs)
	if err != nil {
		return s, err
	}
	if next == nil {
		return s, nil
	}
	ns := p.snapshot(next)
	p.publish(ns, cause, ns.displays(changed))
	return ns, nil
}

// recheck is fs compared with the disk (snapFS.refresh), a refresh numbered so that a call that
// took its ticket before it need not refresh again (S1); the caller holds refreshMu.
func (p *Project) recheck(ctx context.Context, fs *snapFS) (*snapFS, []name, error) {
	p.mu.Lock()
	p.begun++
	n := p.begun
	p.mu.Unlock()
	next, changed, err := fs.refresh(ctx)
	if err != nil {
		return nil, nil, err
	}
	p.mu.Lock()
	p.done = n
	p.mu.Unlock()
	return next, changed, nil
}

// publish numbers s after every snapshot published before it and makes it current, then tells
// every subscriber (API.md S10).
func (p *Project) publish(s *Snapshot, cause Cause, files []string) {
	p.mu.Lock()
	p.seq++
	s.seq, p.cur = p.seq, s
	subs := slices.Clone(p.subs)
	p.mu.Unlock()
	for _, sub := range subs {
		sub.fn(Event{Snapshot: s, Cause: cause, Files: files})
	}
}

// Subscribe calls fn with every snapshot published until the returned function runs or the
// project closes, on the publishing goroutine, which holds the refresh: fn must not call p.
func (p *Project) Subscribe(fn func(Event)) func() {
	_, stop := p.subscribe(fn)
	return stop
}

// subscribe is Subscribe, with the snapshot current when fn joined, the one its first event
// follows; nil after Close.
func (p *Project) subscribe(fn func(Event)) (*Snapshot, func()) {
	sub := &subscriber{fn: fn}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return nil, func() {}
	}
	p.subs = append(p.subs, sub)
	return p.cur, func() {
		p.mu.Lock()
		defer p.mu.Unlock()
		p.subs = slices.DeleteFunc(p.subs, func(s *subscriber) bool { return s == sub })
	}
}

// remember records rev as produced from fs (API.md S4).
func (p *Project) remember(rev string, fs *snapFS) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.hist.add(rev, fs)
}
