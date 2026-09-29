package canon

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fantasim/canonlang/internal/workspace"
)

const revTestProject = "project a {\n  canon: \"0.1\"\n  roots {\n    out: \"out\"\n  }\n}\n"

// API.md S11: a cancelled write waiter returns ctx.Err() promptly instead of blocking on the
// project's write lock, held here by another writer.
func TestWritingBuildCancelledBehindWriter(t *testing.T) {
	p, err := Open("/law", Options{FS: newWriteFS(map[string][]byte{"/law/project.canon": []byte(revTestProject)}, nil)})
	if err != nil {
		t.Fatal(err)
	}
	held, release := make(chan struct{}), make(chan struct{})
	done := make(chan error)
	go func() {
		_, err := p.workspace().Write(context.Background(), workspace.CauseEdit, func(context.Context, *workspace.Snapshot) error {
			close(held)
			<-release
			return nil
		})
		done <- err
	}()
	<-held
	ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	if _, err := p.Build(ctx, BuildOptions{}); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("a writing Build behind another writer: %v", err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

// API.md S10: a writing Build whose ctx is done once its writes are made still returns their
// result, with the revision of the snapshot it published.
func TestRevisionAfterWriteFallsBackWithoutCtx(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	src := "/// A.\npackage a\n\n/// V.\nlet v: Int = 1\n\nemit json { out: \"@out/\" }\n"
	fsys := newWriteFS(map[string][]byte{"/law/project.canon": []byte(revTestProject), "/law/a/a.canon": []byte(src)}, cancel)
	p, err := Open("/law", Options{FS: fsys})
	if err != nil {
		t.Fatal(err)
	}
	before := p.Revision()
	res, err := p.Build(ctx, BuildOptions{})
	if err != nil || ctx.Err() == nil {
		t.Fatalf("Build: %v (ctx %v)", err, ctx.Err())
	}
	if res.Check.Revision == "" || res.Check.Revision != p.Revision() || len(res.Outputs) != 1 {
		t.Errorf("revision %q, now %q, before %q, outputs %+v", res.Check.Revision, p.Revision(), before, res.Outputs)
	}
}
