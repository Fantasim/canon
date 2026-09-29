package workspace

import (
	"cmp"
	"context"
	"slices"
	"sync"

	"github.com/fantasim/canonlang/internal/workspace/safego"
)

// Change is a published snapshot as a watch receives it: why, the files changed, created or
// removed, every absolute name that differs from the snapshot before (S2), or Err, the shared
// watcher's panic (a *safego.PanicError), after which a fresh one keeps watching.
type Change struct {
	Snapshot *Snapshot
	Cause    Cause
	Files    []Path
	Changed  []string
	Relisted []string // the directories among Changed whose listing changed
	Err      error
}

// Path is a file by its display path and its absolute name.
type Path struct {
	Display string
	Abs     string
}

// Watcher is one Watch: the snapshots published since it started, queued, handed to its fn one
// at a time in the order they were published (API.md W13).
type Watcher struct {
	p     *Project
	fn    func(context.Context, Change)
	mu    sync.Mutex
	last  *Snapshot // the latest snapshot queued, the one the next change follows
	queue []Change
	wake  chan struct{}
	done  chan struct{}
	stop  func()
}

// Watch calls fn with each snapshot published, one at a time in order on its own goroutine, until
// ctx is done or the project closes; a watcher shared by all Watches, set up with the first's
// options, keeps the snapshot fresh meanwhile (API.md S1, W12-W16). A panic in fn ends the Watch.
func (p *Project) Watch(ctx context.Context, fn func(context.Context, Change), opt WatchOptions) (*Watcher, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if _, _, err := p.current(); err != nil {
		return nil, err
	}
	w := &Watcher{p: p, fn: fn, wake: make(chan struct{}, 1), done: make(chan struct{})}
	p.startWatching(ctx, opt, w)
	if !w.subscribe() {
		p.stopWatching(w)
		return nil, ErrClosed
	}
	safego.Go(func() error { return w.run(ctx) }, func(error) { w.exit() })
	return w, nil
}

// subscribe makes w a subscriber of its project, the snapshot current then the first one its
// changes follow; false after Close.
func (w *Watcher) subscribe() bool {
	w.mu.Lock() // an event published at once waits for last
	defer w.mu.Unlock()
	cur, stop := w.p.subscribe(w.enqueue)
	w.last, w.stop = cur, stop
	return cur != nil
}

// Done is closed once the watch has stopped, its goroutine and, if it was the last, the shared
// watcher's with it.
func (w *Watcher) Done() <-chan struct{} { return w.done }

// enqueue queues ev, a subscriber of the project: on the publishing goroutine, inside the refresh
// that published, it compares the snapshot with the one before, reading now what that one lacks.
func (w *Watcher) enqueue(ev Event) {
	w.mu.Lock()
	from := w.last
	w.last = ev.Snapshot
	w.mu.Unlock()
	c := ev.Snapshot.since(from, ev.Files)
	c.Snapshot, c.Cause = ev.Snapshot, ev.Cause
	w.push(c)
}

// fail queues the shared watcher's failure (a hub panic), reported on the current snapshot.
func (w *Watcher) fail(err error) {
	if s, _, cerr := w.p.current(); cerr == nil {
		w.push(Change{Snapshot: s, Cause: CauseExternal, Err: err})
	}
}

// push queues c, joined to the change queued before it when joins says so (API.md W15).
func (w *Watcher) push(c Change) {
	w.mu.Lock()
	n := len(w.queue)
	if n > 0 && joins(w.queue[n-1], c) {
		tail := &w.queue[n-1]
		tail.Snapshot = c.Snapshot
		tail.Files = unionPaths(tail.Files, c.Files)
		tail.Changed = union(tail.Changed, c.Changed)
		tail.Relisted = union(tail.Relisted, c.Relisted)
	} else {
		w.queue = append(w.queue, c)
	}
	w.mu.Unlock()
	select {
	case w.wake <- struct{}{}:
	default:
	}
}

// joins reports c merged into tail: two failures are one pending failure, so a hub that keeps
// failing never grows a queue; other changes join for one cause, but for an edit (W15).
func joins(tail, c Change) bool {
	if tail.Err != nil || c.Err != nil {
		return tail.Err != nil && c.Err != nil
	}
	return tail.Cause == c.Cause && c.Cause != CauseEdit
}

// run delivers what is queued, in order, until ctx is done or the project closes.
func (w *Watcher) run(ctx context.Context) error {
	for {
		select {
		case <-w.wake:
		case <-ctx.Done():
			return nil
		case <-w.p.closing:
			return nil
		}
		for c, ok := w.pop(); ok; c, ok = w.pop() {
			if w.ending(ctx) {
				return nil
			}
			w.fn(ctx, c)
			w.p.nudge()
		}
	}
}

func (w *Watcher) pop() (Change, bool) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.queue) == 0 {
		return Change{}, false
	}
	c := w.queue[0]
	w.queue = slices.Delete(w.queue, 0, 1)
	return c, true
}

// exit leaves the project: no more events, and the shared watcher stopped after the last watch.
func (w *Watcher) exit() {
	w.stop()
	w.p.stopWatching(w)
	close(w.done)
}

// ending reports ctx done or the project closed: nothing more is delivered.
func (w *Watcher) ending(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	case <-w.p.closing:
		return true
	default:
		return false
	}
}

// union is the names in a or b, in byte order, once each.
func union(a, b []string) []string {
	return slices.Compact(slices.Sorted(slices.Values(append(slices.Clone(a), b...))))
}

// unionPaths is the paths in a or b, by display path, once each.
func unionPaths(a, b []Path) []Path {
	out := append(slices.Clone(a), b...)
	slices.SortFunc(out, func(x, y Path) int { return cmp.Or(cmp.Compare(x.Display, y.Display), cmp.Compare(x.Abs, y.Abs)) })
	return slices.Compact(out)
}
