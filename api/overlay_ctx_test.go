package canon_test

import (
	"context"
	"errors"
	"testing"
	"time"

	canon "github.com/fantasim/canonlang/api"
)

// overlayWaiting is how long an overlay behind a held write lock must still be waiting.
const overlayWaiting = 100 * time.Millisecond

// boxIs is b/b.canon with its box set to n.
func boxIs(n string) []byte {
	return []byte("/// B.\npackage b\n\nimport a\n\n/// A box.\nlet box: Int = " + n + "\n")
}

// overlayCall is an overlay writer called under ctx.
type overlayCall func(ctx context.Context, p *canon.Project) error

var overlayCalls = map[string]overlayCall{
	"SetOverlay":   func(ctx context.Context, p *canon.Project) error { return p.SetOverlay(ctx, "b/b.canon", boxIs("2")) },
	"ClearOverlay": func(ctx context.Context, p *canon.Project) error { return p.ClearOverlay(ctx, "b/b.canon") },
}

// API.md S9, S11, DECISIONS 344: an overlay waits for the write lock only until its ctx is done.
func TestOverlayWaitsUntilCtxDone(t *testing.T) {
	for _, name := range []string{"SetOverlay", "ClearOverlay"} {
		t.Run(name, func(t *testing.T) { overlayCancelled(t, overlayCalls[name]) })
	}
}

// overlayCancelled cancels call while an edit holds the write lock, and finds the overlay of b as
// it was set before.
func overlayCancelled(t *testing.T, call overlayCall) {
	fsys := &pausingFS{memFS: lawFS(), reached: make(chan struct{}), release: make(chan struct{})}
	p, err := canon.Open("/law", canon.Options{FS: fsys})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	ctx := context.Background()
	if err := p.SetOverlay(ctx, "b/b.canon", boxIs("1")); err != nil {
		t.Fatal(err)
	}
	done, cancel := context.WithCancel(ctx)
	cancel()
	if err := call(done, p); !errors.Is(err, context.Canceled) {
		t.Errorf("API.md S11: a done ctx: %v", err)
	}
	edited := make(chan error, 1)
	go func() {
		_, err := p.Edit(ctx, threeFiles)
		edited <- err
	}()
	<-fsys.reached // the edit holds the write lock (S9)
	waiting, stop := context.WithCancel(ctx)
	returned := make(chan error, 1)
	go func() { returned <- call(waiting, p) }()
	select {
	case err := <-returned:
		t.Fatalf("API.md S9: returned while the lock is held, its ctx live: %v", err)
	case <-time.After(overlayWaiting):
	}
	stop()
	if err := <-returned; !errors.Is(err, context.Canceled) {
		t.Errorf("API.md S11: waiting for the write lock, ctx done: %v", err)
	}
	close(fsys.release)
	if err := <-edited; err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if v, err := p.Value(ctx, "b:box"); err != nil || v.Text != "1" {
		t.Errorf("DECISIONS 344: after a cancelled overlay call, b:box %v, %v; want the overlay set before", v, err)
	}
	if err := call(ctx, p); err != nil {
		t.Errorf("API.md §3.4: the lock free: %v", err)
	}
}
