package workspace

import (
	"cmp"
	"context"
	"crypto/sha256"
	"errors"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/fantasim/canonlang/internal/workspace/safego"
)

// Clock is the time a watch coalesces changes by (API.md W14): the system's, or a test's.
type Clock interface {
	Now() time.Time
	// After is a channel that receives once d has passed, and a function that releases it.
	After(d time.Duration) (<-chan time.Time, func() bool)
}

// WatchOptions set how the shared watcher notices changes; the zero value polls every
// pollEvery by the system clock, unless the file system declares itself the OS's, and logs nothing.
type WatchOptions struct {
	Clock  Clock
	Poll   time.Duration            // how often a file system that is not the OS's is compared
	OS     bool                     // the file system is the OS's: its notifications are followed
	Logger *slog.Logger             // told when the OS's notifications fail and polling replaces them
	notify func() (notifier, error) // the OS's notifications; nil: fsnotify's
}

type systemClock struct{}

func (systemClock) Now() time.Time { return time.Now() }

func (systemClock) After(d time.Duration) (<-chan time.Time, func() bool) {
	t := time.NewTimer(d)
	return t.C, t.Stop
}

// watchSet is the watches of a project and the one watcher they share (API.md W16).
type watchSet struct {
	mu        sync.Mutex // starting and stopping the hub, and the watches
	hub       atomic.Pointer[hub]
	users     int
	watchers  map[*Watcher]bool
	restartAt time.Time // when the last hub replacing a failed one started, by the system clock
}

// running reports a hub keeping the snapshot fresh: reads take it as it is (S1).
func (ws *watchSet) running() bool { return ws.hub.Load() != nil }

// hub notices changes to what the current snapshot read, by the OS's notifications and by
// comparing the snapshot with the disk, coalesces them and refreshes the snapshot (W12, W14).
type hub struct {
	p       *Project
	opt     WatchOptions // what it was started with, which a hub replacing it after a panic reuses
	clk     Clock
	log     *slog.Logger
	every   time.Duration     // how often the snapshot is compared with the disk when polling
	os      notifier          // nil when polling only
	dirs    map[string]bool   // the directories os watches
	covered map[string]string // a directory missing, to the one above it os watches instead
	seen    *snapFS           // the snapshot dirs was made from, at gen seenGn
	seenGn  int
	onOS    bool          // the OS's notifications are followed, unless they fail
	kick    chan struct{} // a delivery asks os to follow what its re-check read; nil when polling
	delay   time.Duration // how long a hub replacing a failed one waits before it starts
	ready   chan struct{} // closed once it watches
	ctx     context.Context
	cancel  context.CancelFunc
	stop    chan struct{}
	done    chan struct{}
	due     time.Time // the next comparison with the disk, and resync of os
	dirty   bool      // a change seen and not yet refreshed, first at first, latest at last
	first   time.Time
	last    time.Time
	polled  [sha256.Size]byte // the changes the last comparison saw, so a new one is told apart
}

// startWatching adds w to the project's watches and starts the hub if none runs (W12, W16).
func (p *Project) startWatching(ctx context.Context, opt WatchOptions, w *Watcher) {
	ws := &p.watches
	ws.mu.Lock()
	defer ws.mu.Unlock()
	if ws.hub.Load() == nil {
		h := p.newHub(ctx, opt, 0)
		ws.hub.Store(h)
		select { // watching, then Watch returns (W12); a panic setting up ends the hub first
		case <-h.ready:
		case <-h.done:
		}
	}
	if ws.watchers == nil {
		ws.watchers = map[*Watcher]bool{}
	}
	ws.users++
	ws.watchers[w] = true
}

// stopWatching removes w; the last watch stops the hub and waits for its goroutine.
func (p *Project) stopWatching(w *Watcher) {
	ws := &p.watches
	ws.mu.Lock()
	defer ws.mu.Unlock()
	delete(ws.watchers, w)
	ws.users--
	if ws.users > 0 {
		return
	}
	if h := ws.hub.Swap(nil); h != nil {
		close(h.stop)
		h.cancel()
		<-h.done // no deadlock: the hub's callback closes done before hubEnded takes ws.mu
	}
}

// hubEnded detaches h once its goroutine is done, unless it was stopped; after a panic, told to
// every watch (X2), a fresh hub replaces it at once, since only ctx or Close ends a watch (W12).
func (p *Project) hubEnded(h *hub, err error) {
	h.cancel()
	ws := &p.watches
	ws.mu.Lock()
	defer ws.mu.Unlock()
	if ws.hub.Load() != h { // every store is under ws.mu
		return
	}
	failed := errors.Is(err, safego.ErrPanic)
	var next *hub
	if failed && ws.users > 0 && !isDone(p.closing) {
		next = p.newHub(h.ctx, h.opt, ws.restartDelay())
	}
	ws.hub.Store(next) // one store: a hub being replaced counts as running throughout (S1, W15)
	if !failed {
		return
	}
	//canon:unordered each watch queues the failure on its own
	for w := range ws.watchers {
		w.fail(err)
	}
}

// restartDelay is how long a hub replacing a failed one waits: none a second after the last
// restart, else until then, so a hub that keeps failing restarts once per resyncEvery.
func (ws *watchSet) restartDelay() time.Duration {
	now := time.Now()
	at := ws.restartAt.Add(resyncEvery)
	if at.Before(now) {
		at = now
	}
	ws.restartAt = at
	return at.Sub(now)
}

func isDone(ch <-chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

// nudge asks the hub to follow what a delivery's re-check read.
func (p *Project) nudge() {
	h := p.watches.hub.Load()
	if h == nil {
		return
	}
	select {
	case h.kick <- struct{}{}:
	default:
	}
}

// newHub starts a hub after delay; its own goroutine sets it up, so a panic doing so is
// recovered as any other of the hub's (X2).
func (p *Project) newHub(ctx context.Context, opt WatchOptions, delay time.Duration) *hub {
	h := &hub{p: p, opt: opt, clk: opt.Clock, log: opt.Logger, every: cmp.Or(opt.Poll, pollEvery), delay: delay}
	h.stop, h.done, h.ready = make(chan struct{}), make(chan struct{}), make(chan struct{})
	if h.clk == nil {
		h.clk = systemClock{}
	}
	if h.log == nil {
		h.log = slog.New(slog.DiscardHandler)
	}
	h.ctx, h.cancel = context.WithCancel(context.WithoutCancel(ctx))
	if h.onOS = opt.OS || osBacked(p.tmpl.FS()); h.onOS {
		h.kick = make(chan struct{}, 1)
	}
	safego.Go(h.run, func(err error) {
		h.closeOS()
		close(h.done)
		p.hubEnded(h, err)
	})
	return h
}

// run sets the hub up, then waits for a change, a deadline or the end, one timer at a time,
// until stopped.
func (h *hub) run() error {
	if !h.pause() {
		return nil
	}
	if h.onOS {
		notify := h.opt.notify
		if notify == nil {
			notify = openNotifier
		}
		h.watchOS(notify)
	}
	close(h.ready)
	for {
		timer, release := h.clk.After(h.wait())
		ok := h.sleep(timer)
		release()
		if !ok {
			return nil
		}
		if err := h.step(); err != nil {
			return err
		}
	}
}

// pause waits the hub's delay by the system clock, which a failing Clock cannot break; false
// when the hub is stopped meanwhile.
func (h *hub) pause() bool {
	if h.delay <= 0 {
		return true
	}
	t := time.NewTimer(h.delay)
	defer t.Stop()
	select {
	case <-t.C:
		return true
	case <-h.stop:
		return false
	case <-h.p.closing:
		return false
	}
}

// sleep waits for the timer, a notification, a kick or the end, false for the end.
func (h *hub) sleep(timer <-chan time.Time) bool {
	var events <-chan fsnotify.Event
	var errs <-chan error
	if h.os != nil {
		events, errs = h.os.events(), h.os.errs()
	}
	select {
	case <-timer:
	case ev, ok := <-events:
		if !ok {
			return false
		}
		h.notified(ev)
	case _, ok := <-errs: // an overflow or a failed read: something may have changed
		if !ok {
			return false
		}
		h.note(h.clk.Now())
	case <-h.kick:
		h.due = h.clk.Now()
	case <-h.stop:
		return false
	case <-h.p.closing:
		return false
	}
	return true
}

// step compares the snapshot with the disk when due, first resyncing the OS watcher, then
// refreshes the snapshot once the changes seen have settled.
func (h *hub) step() error {
	now := h.clk.Now()
	if !now.Before(h.due) {
		if h.os != nil {
			h.resync(now)
		}
		if err := h.pollOnce(now); err != nil {
			return err
		}
		h.due = now.Add(h.interval())
	}
	if !h.ripe(now) {
		return nil
	}
	h.dirty = false
	return h.p.refreshWatched(h.ctx)
}

// interval is how often the snapshot is compared with the disk: every resync of the OS watcher,
// its safety net, or every poll (S1).
func (h *hub) interval() time.Duration {
	if h.os != nil {
		return resyncEvery
	}
	return h.every
}

// note records a change seen at now.
func (h *hub) note(now time.Time) {
	if !h.dirty {
		h.dirty, h.first = true, now
	}
	h.last = now
}

// ripe reports the changes seen settled: quietFor without another, or capFor since the first (W14).
func (h *hub) ripe(now time.Time) bool {
	return h.dirty && (now.Sub(h.last) >= quietFor || now.Sub(h.first) >= capFor)
}

// wait is how long until the next deadline: a comparison, or the changes seen settling.
func (h *hub) wait() time.Duration {
	next := h.due
	if h.dirty {
		next = earliest(next, h.last.Add(quietFor), h.first.Add(capFor))
	}
	return next.Sub(h.clk.Now())
}

func earliest(t time.Time, others ...time.Time) time.Time {
	for _, o := range others {
		if o.Before(t) {
			t = o
		}
	}
	return t
}

// pollOnce compares the current snapshot with the disk as a refresh does, and notes a change
// when what differs is not what differed at the last comparison.
func (h *hub) pollOnce(now time.Time) error {
	s, _, err := h.p.current()
	if err != nil {
		return err
	}
	next, changed, err := s.fs.refresh(h.ctx)
	if err != nil {
		return err
	}
	fp := fingerprint(next, changed)
	if fp != h.polled && len(changed) > 0 {
		h.note(now)
	}
	h.polled = fp
	return nil
}

// fingerprint is the SHA-256 of names and of their content in next; zero for none.
func fingerprint(next *snapFS, names []name) [sha256.Size]byte {
	if len(names) == 0 {
		return [sha256.Size]byte{}
	}
	sum := sha256.New()
	for _, n := range names {
		e := next.ents[n]
		sum.Write([]byte{byte(n.kind), byte(e.sum.class)})
		sum.Write(e.sum.hash[:])
		sum.Write(append([]byte(n.abs), 0))
	}
	return [sha256.Size]byte(sum.Sum(nil))
}

// refreshWatched refreshes the snapshot for the hub as a writer does, so that it never reads a
// writer's own writes before the writer publishes them with its cause (API.md W15).
func (p *Project) refreshWatched(ctx context.Context) error {
	if err := p.lock(ctx); err != nil {
		return err
	}
	defer p.unlock()
	p.refreshMu.Lock()
	defer p.refreshMu.Unlock()
	s, _, err := p.current()
	if err != nil {
		return err
	}
	_, err = p.refresh(ctx, s, CauseExternal)
	return err
}
