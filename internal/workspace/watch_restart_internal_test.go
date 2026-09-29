package workspace

import (
	"context"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/fantasim/canonlang/internal/workspace/safego"
)

// panickyNotifier panics watching a directory while its switch is on, as a bug in a hub would.
type panickyNotifier struct {
	*fakeNotifier
	on *atomic.Bool
}

func (n panickyNotifier) add(name string) error {
	if n.on.Load() {
		panic("resync bug")
	}
	return n.fakeNotifier.add(name)
}

// stormClock is the system clock whose After panics while its switch is on.
type stormClock struct {
	systemClock
	on *atomic.Bool
}

func (c stormClock) After(d time.Duration) (<-chan time.Time, func() bool) {
	if c.on.Load() {
		panic("clock bug")
	}
	return c.systemClock.After(d)
}

// API.md X2 (log-2026-09-29 M4 U5-r5 1): a hub that panics while following a directory, then
// again while the hub replacing it sets up, is two recovered failures, never a crash; once the
// bug is gone the watch keeps watching.
func TestWatchRestartSetupPanics(t *testing.T) {
	dir, p := osLaw(t)
	var on atomic.Bool
	notify := func() (notifier, error) {
		return panickyNotifier{&fakeNotifier{ev: make(chan fsnotify.Event), er: make(chan error)}, &on}, nil
	}
	changes := make(chan Change, 16)
	ctx, cancel := context.WithCancel(context.Background())
	w, err := p.Watch(ctx, func(ctx context.Context, c Change) {
		_, _ = c.Snapshot.Revision(ctx) // reads e/: the resync that follows it panics
		changes <- c
	}, WatchOptions{OS: true, notify: notify})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { cancel(); <-w.Done() }()
	on.Store(true)
	writeFile(t, filepath.Join(dir, "e", "e.canon"), "/// E.\npackage e\n")
	waitFailure(t, changes) // following e/
	waitFailure(t, changes) // setting up the hub replacing it
	on.Store(false)
	writeFile(t, filepath.Join(dir, "a", "a.canon"), "/// A.\npackage a\n\n/// N.\nconst N = 1\n")
	waitChange(t, changes, "a/a.canon", true)
}

// API.md X2 (log-2026-09-29 M4 U5-r5 2): a hub that fails for good restarts at most once
// a second, and its failures wait as one change behind a slow delivery: bounded CPU and memory.
func TestWatchHubStorm(t *testing.T) {
	_, p := osLaw(t)
	var on atomic.Bool
	var fails atomic.Int32
	release := make(chan struct{})
	ctx, cancel := context.WithCancel(context.Background())
	w, err := p.Watch(ctx, func(context.Context, Change) {
		if fails.Add(1) == 1 {
			<-release
		}
	}, WatchOptions{Clock: stormClock{on: &on}, Poll: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { cancel(); <-w.Done() }()
	start := time.Now()
	on.Store(true)
	queued := 0
	for time.Since(start) < stormFor {
		w.mu.Lock()
		queued = max(queued, len(w.queue))
		w.mu.Unlock()
		time.Sleep(time.Millisecond)
	}
	close(release)
	time.Sleep(stormSettle)
	elapsed := time.Since(start)
	if queued > 1 {
		t.Errorf("%d failures queued behind a slow delivery, want at most 1", queued)
	}
	if limit := 2 + int32(elapsed/resyncEvery); fails.Load() > limit {
		t.Errorf("%d failures delivered in %v, want at most %d", fails.Load(), elapsed, limit)
	}
}

// stormFor is how long the storm lasts, stormSettle how long its last failure takes to arrive.
const (
	stormFor    = 2500 * time.Millisecond
	stormSettle = 100 * time.Millisecond
)

// waitFailure waits for a change carrying the hub's recovered panic.
func waitFailure(t *testing.T, changes <-chan Change) {
	t.Helper()
	deadline := time.After(20 * time.Second)
	for {
		select {
		case c := <-changes:
			if errors.Is(c.Err, safego.ErrPanic) {
				return
			}
		case <-deadline:
			t.Fatal("no failure change")
		}
	}
}
