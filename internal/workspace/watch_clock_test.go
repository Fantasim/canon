package workspace_test

import (
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeClock is a Clock that moves only when a test advances it; the hub arms one timer at a time,
// so one pending timer means the hub waits, done with everything before it.
type fakeClock struct {
	mu        sync.Mutex
	now       time.Time
	timers    []*fakeTimer
	boom      atomic.Bool  // the next After panics, as a bug in the hub would
	exhausted func() error // the machine's inotify limit the hub fell back for, nil while it runs
}

type fakeTimer struct {
	at time.Time
	c  chan time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{now: time.Unix(1_000_000, 0)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

func (c *fakeClock) After(d time.Duration) (<-chan time.Time, func() bool) {
	if c.boom.CompareAndSwap(true, false) {
		panic("clock bug")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	t := &fakeTimer{at: c.now.Add(d), c: make(chan time.Time, 1)}
	if d <= 0 {
		t.c <- c.now
		return t.c, func() bool { return false }
	}
	c.timers = append(c.timers, t)
	return t.c, func() bool {
		c.mu.Lock()
		defer c.mu.Unlock()
		n := len(c.timers)
		c.timers = slices.DeleteFunc(c.timers, func(o *fakeTimer) bool { return o == t })
		return len(c.timers) < n
	}
}

// advance moves the clock by d and fires every timer due.
func (c *fakeClock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = c.now.Add(d)
	c.timers = slices.DeleteFunc(c.timers, func(t *fakeTimer) bool {
		if t.at.After(c.now) {
			return false
		}
		t.c <- c.now
		return true
	})
}

// idle waits until the hub has armed its next timer: every step before it is done.
func (c *fakeClock) idle(t *testing.T) {
	t.Helper()
	if !c.idleWithin(waitLimit) {
		t.Fatal("the watcher never went idle")
	}
}

// idleWithin reports the hub idle within d of real time.
func (c *fakeClock) idleWithin(d time.Duration) bool {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		c.mu.Lock()
		n := len(c.timers)
		c.mu.Unlock()
		if n == 1 {
			return true
		}
		time.Sleep(time.Millisecond)
	}
	return false
}

// skipIfExhausted skips the test once the hub fell back to polling for want of inotify watches or
// instances (the machine's per-user limits, shared with every editor and test on it): the OS
// notifications these tests exist to prove cannot run there.
func (c *fakeClock) skipIfExhausted(t *testing.T) {
	t.Helper()
	if c.exhausted == nil {
		return
	}
	if err := c.exhausted(); err != nil {
		t.Skipf("the OS cannot notify changes here: %v", err)
	}
}

// armedIn waits until the hub waits for exactly d from now: a change noted now, when its next
// comparison with the disk is further away.
func (c *fakeClock) armedIn(t *testing.T, d time.Duration) {
	t.Helper()
	deadline := time.Now().Add(waitLimit)
	for time.Now().Before(deadline) {
		c.mu.Lock()
		ok := len(c.timers) == 1 && c.timers[0].at.Equal(c.now.Add(d))
		c.mu.Unlock()
		if ok {
			return
		}
		c.skipIfExhausted(t)
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("the watcher never waited %v", d)
}

// step advances the clock by d, then waits for the hub to be idle again.
func (c *fakeClock) step(t *testing.T, d time.Duration) {
	t.Helper()
	c.advance(d)
	c.idle(t)
}

// waitLimit bounds every wait of these tests, generous for -race on a loaded machine.
const waitLimit = 20 * time.Second
