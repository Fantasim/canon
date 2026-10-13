package writelock

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// The waits of these tests: one a lock that is held must outlast, and the bound of a prompt return.
const (
	held    = 350 * time.Millisecond
	prompt  = time.Second
	rule    = "DECISIONS 352: "
	pkgA    = "a"
	pkgB    = "b"
	cycles  = 40
	workers = 4
)

func needLocks(t *testing.T) {
	t.Helper()
	if !lockable() {
		t.Skip("no lock primitive on this platform: the lock is skipped")
	}
}

// acquire is Acquire of reqs in dir, failing the test on an error.
func acquire(t *testing.T, dir string, reqs ...Request) *Held {
	t.Helper()
	h, err := Acquire(context.Background(), dir, reqs)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = h.Release() })
	return h
}

// within is Acquire of reqs in dir given d to succeed: its error, nil when it took them.
func within(dir string, d time.Duration, reqs ...Request) error {
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()
	h, err := Acquire(ctx, dir, reqs)
	if err == nil {
		return h.Release()
	}
	return err
}

func ex(pkg string) Request { return Request{Package: pkg, Mode: Exclusive} }
func sh(pkg string) Request { return Request{Package: pkg, Mode: Shared} }

// DECISIONS 352: an exclusive holder excludes an exclusive and a shared writer until it releases.
func TestExclusiveExcludes(t *testing.T) {
	needLocks(t)
	dir := t.TempDir()
	h := acquire(t, dir, ex(pkgA))
	for _, r := range []Request{ex(pkgA), sh(pkgA)} {
		if err := within(dir, held, r); !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("%smode %d while held exclusively: %v", rule, r.Mode, err)
		}
	}
	if err := within(dir, held, ex(pkgB)); err != nil {
		t.Errorf("%sanother package: %v", rule, err)
	}
	if err := h.Release(); err != nil {
		t.Fatal(err)
	}
	if err := within(dir, prompt, ex(pkgA)); err != nil {
		t.Errorf("%safter release: %v", rule, err)
	}
}

// DECISIONS 352: shared holders admit each other and exclude an exclusive writer.
func TestSharedAdmitsShared(t *testing.T) {
	needLocks(t)
	dir := t.TempDir()
	first := acquire(t, dir, sh(pkgA))
	second := acquire(t, dir, sh(pkgA))
	if err := within(dir, held, ex(pkgA)); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("%sexclusive among shared holders: %v", rule, err)
	}
	if err := errors.Join(first.Release(), second.Release()); err != nil {
		t.Fatal(err)
	}
	if err := within(dir, prompt, ex(pkgA)); err != nil {
		t.Errorf("%safter both released: %v", rule, err)
	}
}

// DECISIONS 352: a context cancelled while waiting returns promptly, its error kept.
func TestCancelWhileWaiting(t *testing.T) {
	needLocks(t)
	dir := t.TempDir()
	acquire(t, dir, ex(pkgA))
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(pollEvery/2, cancel)
	start := time.Now()
	_, err := Acquire(ctx, dir, []Request{ex(pkgA)})
	if !errors.Is(err, context.Canceled) || time.Since(start) > prompt {
		t.Errorf("%scancelled after %v: %v", rule, time.Since(start), err)
	}
}

// DECISIONS 352: a waiting writer is told once whom it waits for, the holder's "host pid".
func TestWaitingToldOnce(t *testing.T) {
	needLocks(t)
	dir := t.TempDir()
	acquire(t, dir, ex(pkgA))
	host, err := os.Hostname()
	if err != nil {
		host = unknownHost
	}
	var told []Wait
	ctx, cancel := context.WithTimeout(WithWaiting(context.Background(), func(w Wait) { told = append(told, w) }), held)
	defer cancel()
	if _, err := Acquire(ctx, dir, []Request{ex(pkgA)}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal(err)
	}
	want := Wait{Package: pkgA, Holder: fmt.Sprintf("%s %d", host, os.Getpid())}
	if len(told) != 1 || told[0] != want {
		t.Errorf("%stold %v, want once %v", rule, told, want)
	}
	data, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(writersPath), lockPrefix+pkgA))
	if err != nil || string(data) != want.Holder+lineEnd {
		t.Errorf("%sthe lock file holds %q (%v)", rule, data, err)
	}
}

// DECISIONS 352: all or none: a set whose last lock is held leaves its first free again.
func TestAllOrNone(t *testing.T) {
	needLocks(t)
	dir := t.TempDir()
	acquire(t, dir, ex(pkgB))
	if err := within(dir, held, ex(pkgB), ex(pkgA)); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("%s%v", rule, err)
	}
	if err := within(dir, held, ex(pkgA)); err != nil {
		t.Errorf("%sthe first lock stayed held: %v", rule, err)
	}
}

// DECISIONS 352: a package asked twice is held once, in the stronger mode.
func TestStrongerModeWins(t *testing.T) {
	needLocks(t)
	dir := t.TempDir()
	h := acquire(t, dir, sh(pkgA), ex(pkgA), sh(pkgB))
	if !h.Holds(pkgA, Exclusive) || !h.Holds(pkgB, Shared) || h.Holds(pkgB, Exclusive) || h.Holds("c", Shared) {
		t.Errorf("%sholds %v", rule, h.reqs)
	}
	if err := within(dir, held, sh(pkgA)); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("%sshared beside the stronger mode: %v", rule, err)
	}
}

// DECISIONS 352: in-process writers taking two locks in opposite orders never deadlock.
func TestOppositeOrdersInProcess(t *testing.T) {
	needLocks(t)
	dir := t.TempDir()
	done := make(chan error, workers)
	for i := range workers {
		reqs := []Request{ex(pkgA), ex(pkgB)}
		if i%2 == 1 {
			reqs = []Request{ex(pkgB), sh(pkgA)}
		}
		go func() {
			for range cycles {
				if err := within(dir, time.Minute, reqs...); err != nil {
					done <- err
					return
				}
			}
			done <- nil
		}()
	}
	for range workers {
		if err := <-done; err != nil {
			t.Errorf("%s%v", rule, err)
		}
	}
}
