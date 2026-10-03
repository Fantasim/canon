package workspace_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fantasim/canonlang/internal/workspace"
	"github.com/fantasim/canonlang/internal/workspace/safego"
)

// waiting is how long an overlay writer must still be waiting for a held lock.
const waiting = 100 * time.Millisecond

// API.md S9, S11: an overlay writer waits for the one write lock only until its ctx is done; a
// done ctx returns before anything, and once the lock is free the write goes through.
func TestOverlayWritersTakeCtx(t *testing.T) {
	p := open(t, newMemFS(lawFiles()))
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	for _, err := range []error{
		p.SetOverlayContext(cancelled, "b/b.canon", []byte(srcB)), p.ClearOverlayContext(cancelled, "b/b.canon"),
	} {
		if !errors.Is(err, context.Canceled) {
			t.Errorf("a done ctx: %v", err)
		}
	}
	held, release, wrote := make(chan struct{}), make(chan struct{}), make(chan error, 1)
	safego.Go(func() error {
		_, err := p.Write(context.Background(), workspace.CauseEdit, func(context.Context, *workspace.Snapshot) error {
			close(held)
			<-release
			return nil
		})
		return err
	}, func(err error) { wrote <- err })
	<-held
	ctx, stop := context.WithCancel(context.Background())
	done := make(chan error, 1)
	safego.Go(func() error { return p.SetOverlayContext(ctx, "b/b.canon", []byte(srcB+"\n")) }, func(err error) { done <- err })
	select {
	case err := <-done:
		t.Fatalf("returned while the lock is held, its ctx live: %v", err)
	case <-time.After(waiting):
	}
	stop()
	if err := waitErr(t, done); !errors.Is(err, context.Canceled) {
		t.Errorf("waiting for the write lock, ctx done: %v", err)
	}
	close(release)
	if err := waitErr(t, wrote); err != nil {
		t.Fatal(err)
	}
	if err := p.ClearOverlayContext(context.Background(), "b/b.canon"); err != nil {
		t.Errorf("the lock free: %v", err)
	}
	if err := p.SetOverlayContext(context.Background(), "b/b.canon", []byte(srcB+"\n")); err != nil {
		t.Errorf("the lock free: %v", err)
	}
}
