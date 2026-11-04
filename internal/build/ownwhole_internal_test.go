package build

import (
	"context"
	"testing"
)

// API.md F7, E18 (DECISIONS 463): OwnWhole reads every finding past Options.MaxFindings and leaves
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
	if whole := a.OwnWhole(); len(whole) != 2 {
		t.Errorf("OwnWhole: %v, want both findings", whole)
	}
	if own := a.Own(); len(own.List) != 1 || own.Summary.Errors != 2 || len(own.Summary.Truncated) != 1 {
		t.Errorf("Own after OwnWhole: %v %+v, want one kept of two", own.List, own.Summary)
	}
	if kept := a.r.s.bag("a").Findings(); len(kept) != 1 {
		t.Errorf("the bag after OwnWhole keeps %d, want its cut of 1 back", len(kept))
	}
}
