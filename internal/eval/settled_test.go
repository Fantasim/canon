package eval_test

import (
	"context"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
)

// CLI.md §3.7, EVALUATION.md §9.3: Produced is each value as its origin produced it.
func TestProducedBeforeAmendments(t *testing.T) {
	ctx := context.Background()
	fs := &source.FileSet{}
	srcs := map[string]string{
		"s/s.canon": "/// S.\npackage s\n\n/// Server.\nrecord Server {\n  /// Port.\n  port: Int = 8765\n}\n\n" +
			"/// Config.\nrecord Config {\n  /// Server.\n  server: Server = {}\n}\n\n/// Config.\nlet config: Config = {}\n",
		"s/l.layer.canon": "package s\nlayer l\n\namend config {\n  server.port: 9000\n}\n",
	}
	var files []*syntax.File
	for _, name := range []string{"s/s.canon", "s/l.layer.canon"} {
		src, err := fs.Add(name, "/"+name, []byte(srcs[name]))
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, syntax.Parse(src, syntax.FileSource, diag.NewBag(fs, "")))
	}
	bags, opt := check.Bags{}, eval.Options{Layers: []string{"l"}}
	prog := check.Check(ctx, project.New("demo", project.Version{Minor: 1}), files, bags, eval.NewFolder(bags, opt))
	ev := eval.New(prog, served{}, bags, opt)
	config, ok := ev.Force(ctx, eval.Root{Pkg: "s", Name: "config"})
	if !ok {
		t.Fatal("config did not force")
	}
	server := config.(*value.Record).Fields[0]
	port := server.(*value.Record).Fields[0]
	for _, c := range []struct {
		v    value.Value
		want string
	}{
		{config, "Config{server: Server{port: 8765}}"},
		{server, "Server{port: 8765}"},
		{port, "9000"},
	} {
		if got := ev.Produced(c.v).CanonText(); got != c.want {
			t.Errorf("Produced(%s) = %s, want %s", c.v.CanonText(), got, c.want)
		}
	}
}

// EVALUATION.md §3.1: Settled reads a root only once forcing completed, and never forces one.
func TestSettledNeverEvaluates(t *testing.T) {
	ctx := context.Background()
	fs := &source.FileSet{}
	src, err := fs.Add("a/a.canon", "/a/a.canon", []byte("/// A.\npackage a\n\n/// Good.\nlet good: Int = 6 * 7\n\n"+
		"/// Bad.\nlet bad: Int = [1][3]\n\n/// Twice.\nfn twice(n: Int) -> Int {\n  return n * 2\n}\n"))
	if err != nil {
		t.Fatal(err)
	}
	files := []*syntax.File{syntax.Parse(src, syntax.FileSource, diag.NewBag(fs, ""))}
	bags := check.Bags{}
	prog := check.Check(ctx, project.New("demo", project.Version{Minor: 1}), files, bags, eval.NewFolder(bags, eval.Options{}))
	ev := eval.New(prog, served{}, bags, eval.Options{})
	good, bad := eval.Root{Pkg: "a", Name: "good"}, eval.Root{Pkg: "a", Name: "bad"}
	for _, r := range []eval.Root{good, bad, {Pkg: "a", Name: "twice"}, {Pkg: "z", Name: "good"}} {
		if v, ok := ev.Settled(r); ok || v != nil {
			t.Errorf("before forcing, Settled(%v) = %v, %t", r, v, ok)
		}
	}
	if n := len(bags["a"].Findings()); n != 0 {
		t.Fatalf("Settled evaluated: %d findings", n)
	}
	ev.Force(ctx, good)
	ev.Force(ctx, bad)
	if v, ok := ev.Settled(good); !ok || v.CanonText() != "42" {
		t.Errorf("Settled(good) = %v, %t, want 42", v, ok)
	}
	if v, ok := ev.Settled(bad); ok || v != nil {
		t.Errorf("Settled(bad) = %v, %t: a poisoned root has no value", v, ok)
	}
}
