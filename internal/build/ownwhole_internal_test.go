package build

import (
	"context"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
)

// API.md F7, E18 (DECISIONS 463): ownWhole reads every finding past Options.MaxFindings and leaves
// the cut the analysis reports as it was.
func TestOwnWholePastTheCut(t *testing.T) {
	fsys := roFS{
		"law/project.canon": srcFile("project acme {\n  canon: \"0.1\"\n}\n"),
		"law/a/a.canon":     srcFile("/// A.\npackage a\n\n/// X.\nlet x: Int(1..) = 0\n\n/// Y.\nlet y: Int(1..) = 0\n"),
	}
	p, err := Open(fsys, "/law", Options{MaxFindings: 1})
	if err != nil {
		t.Fatal(err)
	}
	a, err := p.AnalyzeOnly(context.Background(), []string{"a"})
	if err != nil {
		t.Fatal(err)
	}
	if whole := a.ownWhole(); len(whole) != 2 {
		t.Errorf("ownWhole: %v, want both findings", whole)
	}
	if own := a.Own(); len(own.List) != 1 || own.Summary.Errors != 2 || len(own.Summary.Truncated) != 1 {
		t.Errorf("Own after ownWhole: %v %+v, want one kept of two", own.List, own.Summary)
	}
	if kept := a.r.s.bag("a").Findings(); len(kept) != 1 {
		t.Errorf("the bag after ownWhole keeps %d, want its cut of 1 back", len(kept))
	}
}

// API.md F7, R2 (DECISIONS 196, 469): resultWhole is Result's findings uncut, the errors of a
// package only imported included past its cut, and reading it leaves every bag's cut as it was.
func TestResultWholeImportedPastTheCut(t *testing.T) {
	fsys := roFS{
		"law/project.canon": srcFile("project acme {\n  canon: \"0.1\"\n}\n"),
		"law/a/a.canon":     srcFile("/// A.\npackage a\n\nimport b\n\n/// S.\nlet s: Int = b.x + b.y\n"),
		"law/b/b.canon":     srcFile("/// B.\npackage b\n\n/// X.\nlet x: Int(1..) = 0\n\n/// Y.\nlet y: Int(1..) = 0\n"),
	}
	p, err := Open(fsys, "/law", Options{MaxFindings: 1})
	if err != nil {
		t.Fatal(err)
	}
	a, err := p.Analyze(context.Background(), []string{"a"})
	if err != nil {
		t.Fatal(err)
	}
	inB := func(fs []diag.Finding) int {
		return len(slices.DeleteFunc(slices.Clone(fs), func(f diag.Finding) bool { return f.Package != "b" }))
	}
	if n := inB(a.resultWhole()); n != 2 {
		t.Errorf("resultWhole holds %d of b's errors, want both", n)
	}
	if n := inB(a.Result().List); n != 1 {
		t.Errorf("Result holds %d of b's errors, want its cut of 1", n)
	}
	if n := inB(a.ownWhole()); n != 0 {
		t.Errorf("ownWhole holds %d of b's errors, want none: b is only imported", n)
	}
	if kept := a.r.s.bag("b").Findings(); len(kept) != 1 {
		t.Errorf("b's bag after resultWhole keeps %d, want its cut of 1", len(kept))
	}
}
