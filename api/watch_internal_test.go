package canon

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/syntax"
)

// API.md X2: a re-check that panics is an event whose Err is an *InternalError, and the
// watch goes on to the next change.
func TestWatchRecheckPanics(t *testing.T) {
	fsys := newWriteFS(map[string][]byte{
		"/law/project.canon": []byte("project a {\n  canon: \"0.1\"\n}\n"),
		"/law/x/x.canon":     []byte("package x\n"),
	}, func() {})
	panics := func(context.Context, *project.Project, []*syntax.File, map[string]*diag.Bag) *check.Program {
		panic("checker bug")
	}
	b, err := build.Open(fsys, "/law", build.Options{Checker: panics})
	if err != nil {
		t.Fatal(err)
	}
	p := &Project{root: "/law", b: b}
	t.Cleanup(func() { _ = p.Close() })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	events := make(chan Event, 8)
	if err := p.Watch(ctx, func(ev Event) { events <- ev }); err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"package x\n\n", "package x\n\n\n"} {
		if err := fsys.WriteFile("/law/x/x.canon", []byte(text)); err != nil {
			t.Fatal(err)
		}
		select {
		case ev := <-events:
			var ierr *InternalError
			if !errors.As(ev.Err, &ierr) || ierr.Msg != "checker bug" || !slices.Equal(ev.Packages, []string{"x"}) {
				t.Errorf("event: %v, packages %v", ev.Err, ev.Packages)
			}
		case <-time.After(20 * time.Second):
			t.Fatal("no event")
		}
	}
}
