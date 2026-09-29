package control_test

import (
	"context"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/views/control"
)

// noFold folds nothing: this fixture has no consts to fold (IMPLEMENTATION-PLAN §4.7).
type noFold struct{}

func (noFold) Fold(context.Context, check.Object, syntax.Expr, *check.Info) (value.Value, bool) {
	return nil, false
}

// VIEWMODEL.md J4: Index.Broken stays broken under Truncate(1) too (ADR-0009).
func TestIndexBrokenIndependentOfTruncation(t *testing.T) {
	fs := &source.FileSet{}
	src, err := fs.Add("a/a.canon", "/a/a.canon", []byte(`package a

record Thing {
  name: String
}

view Thing {
  title "{missing1} {missing2}"
}
`))
	if err != nil {
		t.Fatal(err)
	}
	bag := diag.NewBag(fs, "a")
	file := syntax.Parse(src, syntax.FileSource, bag)
	bags := check.Bags{"a": bag}
	proj := project.New("t", project.Version{Major: 0, Minor: 1})
	prog := check.Check(context.Background(), proj, []*syntax.File{file}, bags, noFold{})
	if prog == nil {
		t.Fatal("check.Check returned nil")
	}
	var thing types.Type
	for _, o := range prog.Packages[0].Decls {
		if o.Kind() == check.ObjTypeName && o.Name() == "Thing" {
			thing = o.Type()
		}
	}
	if thing == nil {
		t.Fatal("Thing not found among the package's declarations")
	}
	if got := len(bag.Findings()); got < 2 {
		t.Fatalf("want at least 2 findings before truncation, got %d", got)
	}
	if !control.NewIndex(prog, "").Broken(thing) {
		t.Error("Thing's view: want broken (two unresolved names in its title)")
	}
	bag.Truncate(1)
	if got := len(bag.Findings()); got != 1 {
		t.Fatalf("Truncate(1): want 1 finding, got %d", got)
	}
	if !control.NewIndex(prog, "").Broken(thing) {
		t.Error("Thing's view: want still broken after Truncate(1)")
	}
}
