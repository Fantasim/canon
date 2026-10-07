package workspace_test

import (
	"context"
	"errors"
	"fmt"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/workspace"
)

func Example() {
	fsys := newMemFS(map[string]string{
		"/law/project.canon": "project a {\n  canon: \"0.1\"\n}\n",
		"/law/x/x.canon":     "/// X.\npackage x\n\n/// N.\nconst N = 1\n",
	})
	b, err := build.Open(fsys, "/law", build.Options{})
	if err != nil {
		fmt.Println(err)
		return
	}
	p := workspace.New(b)
	defer p.Close()
	stop := p.Subscribe(func(e workspace.Event) { fmt.Println(e.Cause, e.Files) })
	defer stop()
	ctx := context.Background()
	s, err := p.Read(ctx)
	if err != nil {
		return
	}
	a, err := workspace.Share(ctx, s, workspace.Key(workspace.OpAnalyze, nil), func(ctx context.Context) (*build.Analysis, error) {
		return s.Build().Analyze(ctx, nil)
	})
	if err != nil {
		return
	}
	base, _ := s.Revision(ctx)
	fmt.Println(a.Result().Summary.Errors, len(a.Reads("x")))
	_ = p.SetOverlay("x/x.canon", []byte("/// X.\npackage x\n\n/// N.\nconst N = 2\n"))
	now, _ := p.Read(ctx)
	var stale *workspace.StaleError
	isStale := errors.As(now.Stale(base, a.Reads("x")), &stale)
	p.Close()
	_, err = p.Read(ctx)
	fmt.Println(isStale, stale.Files, errors.Is(err, workspace.ErrClosed))
	// Output: 0 5
	// overlay [x/x.canon]
	// true [x/x.canon] true
}
