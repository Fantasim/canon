package workspace

import (
	"context"
	"testing"
	"testing/fstest"

	"github.com/fantasim/canonlang/internal/build"
)

// selectionFS is a imported by b, with a layer of a's.
func selectionFS() fstest.MapFS {
	return fstest.MapFS{
		"law/project.canon": {Data: []byte("project acme {\n  canon: \"0.1\"\n}\n")},
		"law/a/a.canon": {Data: []byte("/// A.\npackage a\n\n/// A config.\nrecord Cfg {\n  /// Port.\n  port: Int\n}\n\n" +
			"/// The config.\nlet cfg: Cfg = { port: 1 }\n")},
		"law/a/live.layer.canon": {Data: []byte("package a\nlayer live\n\namend cfg {\n  port: 3\n}\n")},
		"law/b/b.canon":          {Data: []byte("/// B.\npackage b\n\nimport a\n\n/// Its port.\nlet port: Int = a.cfg.port\n")},
	}
}

// selectionOn is the current snapshot of selectionFS with layers active.
func selectionOn(t *testing.T, layers ...string) *Snapshot {
	t.Helper()
	b, err := build.Open(&clockFS{MapFS: selectionFS()}, "/law", build.Options{Layers: layers})
	if err != nil {
		t.Fatal(err)
	}
	p := New(b)
	t.Cleanup(p.Close)
	s, err := p.Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// API.md E18, W13: the AnalyzeOnly of exactly some packages kept on a snapshot stands for Analyze
// of them, named in any order, but not of others, nor under an active layer (E1901).
func TestKeptSelection(t *testing.T) {
	ctx := context.Background()
	for _, layers := range [][]string{nil, {"live"}} {
		s := selectionOn(t, layers...)
		if KeptSelection(s, []string{"a", "b"}) != nil {
			t.Fatal("an analysis stands for a selection before any was kept")
		}
		a, err := AnalyzeOnly(ctx, s, []string{"a", "b"})
		if err != nil {
			t.Fatal(err)
		}
		want := a
		if len(layers) > 0 {
			want = nil
		}
		if got := KeptSelection(s, []string{"b", "a"}); got != want {
			t.Errorf("layers %v: KeptSelection %p, want %p", layers, got, want)
		}
		for _, other := range [][]string{{"a"}, {"a", "a", "b"}, {"a", "b", "c"}} {
			if KeptSelection(s, other) != nil {
				t.Errorf("layers %v: the analysis of a and b stands for %v", layers, other)
			}
		}
	}
}
