package build

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fantasim/canonlang/internal/load"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/value"
)

var errPanicked = errors.New("load panicked")

// semHost is a run's host whose loads take sem, its read log made.
func semHost(sem loadSem) *evalHost {
	h := &evalHost{loader: &load.Loader{FS: roFS{}, Set: &source.FileSet{}}, loadLock: sem}
	h.track("a")()
	return h
}

// noLoad is a load that finds nothing.
func noLoad() (value.Value, bool, error) { return nil, false, nil }

// API.md X2, S11 (log-2026-09-29 M4 U8-r): a load that panics frees the semaphore and its depth.
func TestLoadPanicFreesSemaphore(t *testing.T) {
	sem := make(loadSem, 1)
	h := semHost(sem)
	func() {
		defer func() { _ = recover() }()
		_, _, _ = h.locked(context.Background(), func() (value.Value, bool, error) { panic(errPanicked) })
	}()
	ctx, cancel := context.WithTimeout(context.Background(), prompt)
	defer cancel()
	if _, _, err := semHost(sem).locked(ctx, noLoad); err != nil {
		t.Fatalf("a load after a panicking one: %v", err)
	}
	if d := h.log().depth; d != 0 {
		t.Errorf("depth %d after the panic", d)
	}
}

// API.md S11 (log-2026-09-29 M4 U8-r): a run waiting for another's loads returns ctx.Err() promptly.
func TestLoadWaitHonoursContext(t *testing.T) {
	sem := make(loadSem, 1)
	holding, release := make(chan struct{}), make(chan struct{})
	go func() {
		_, _, _ = semHost(sem).locked(context.Background(), func() (value.Value, bool, error) {
			close(holding)
			<-release
			return nil, false, nil
		})
	}()
	<-holding
	defer close(release)
	ctx, cancel := context.WithCancel(context.Background())
	time.AfterFunc(time.Millisecond, cancel)
	start := time.Now()
	w := semHost(sem)
	if _, _, err := w.locked(ctx, noLoad); !errors.Is(err, context.Canceled) || time.Since(start) > prompt {
		t.Fatalf("a cancelled wait: %v after %v", err, time.Since(start))
	}
	if d := w.log().depth; d != 0 {
		t.Errorf("depth %d after a cancelled wait", d)
	}
}
