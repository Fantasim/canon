package canon

import (
	"context"
	"errors"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
)

const revTestProject = "project a {\n  canon: \"0.1\"\n}\n"

// API.md S11: a cancelled write waiter returns ctx.Err() promptly instead of blocking on the
// project's write lock.
func TestAcquireWriteCancelled(t *testing.T) {
	p := &Project{}
	if err := p.acquireWrite(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer p.releaseWrite()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := p.acquireWrite(ctx); !errors.Is(err, context.Canceled) {
		t.Errorf("acquireWrite with the lock held: %v", err)
	}
}

// API.md S10: the revision after a write falls back to a ctx-free read instead of losing a
// successful build's result when the re-read's ctx is done.
func TestRevisionAfterWriteFallsBackWithoutCtx(t *testing.T) {
	fsys := newMapFS(map[string][]byte{
		"/law/project.canon": []byte(revTestProject),
		"/law/a/a.canon":     []byte("package a\n"),
	})
	b, err := build.Open(fsys, "/law", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if rev := revisionAfterWrite(ctx, b, "fallback"); rev == "fallback" || rev == "" {
		t.Errorf("revisionAfterWrite with a cancelled ctx = %q, want a fresh revision", rev)
	}
}
