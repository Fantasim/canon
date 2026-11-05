package build

import (
	"context"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
)

// selectionFS is a package a with a layer live, imported by b, and a package c apart.
var selectionFS = roFS{
	"law/project.canon":      srcFile("project acme {\n  canon: \"0.1\"\n}\n"),
	"law/a/a.canon":          srcFile("/// A.\npackage a\n\n/// A config.\nrecord Cfg {\n  /// Port.\n  port: Int\n}\n\n/// The config.\nlet cfg: Cfg = { port: 1 }\n"),
	"law/a/live.layer.canon": srcFile("package a\nlayer live\n\namend cfg {\n  port: 3\n}\n"),
	"law/b/b.canon":          srcFile("/// B.\npackage b\n\nimport a\n\n/// Its port.\nlet port: Int = a.cfg.port\n"),
	"law/c/c.canon":          srcFile("/// C.\npackage c\n\n/// A count.\nlet n: Int = 1\n"),
}

// layersAnalysis is selectionFS analysed with layers active.
func layersAnalysis(t *testing.T, layers ...string) *Analysis {
	t.Helper()
	p, err := Open(selectionFS, "/law", Options{Layers: layers})
	if err != nil {
		t.Fatal(err)
	}
	a, err := p.Analyze(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// API.md F7 (DECISIONS 463): the merged findings are cut once as the bags cut them: each package at
// the session's limit, the project's own, package "", at its own bag's.
func TestCutOnceOwnLimit(t *testing.T) {
	base := layersAnalysis(t)
	own, a := diag.NewBag(base.Files(), ""), diag.NewBag(base.Files(), "a")
	for _, msg := range []string{"one", "two", "three"} {
		diag.E5001.At(source.Span{}, msg).Report(own)
	}
	for _, msg := range []string{"four", "five"} {
		diag.W5001.At(source.Span{}, msg).Report(a)
	}
	m := &merge{parts: []*Analysis{base}}
	for _, f := range append(own.Findings(), a.Findings()...) {
		m.entries = append(m.entries, merged{f: f})
	}
	kept, sum := m.cut(1, 2)
	want := []diag.Truncation{{Package: "", Errors: 1}, {Package: "a", Warnings: 1}}
	if len(kept[0]) != 3 || sum.Errors != 3 || sum.Warnings != 2 || !slices.Equal(sum.Truncated, want) {
		t.Errorf("kept %d, summary %+v: want two of the project's and one of a's kept, the rest counted", len(kept[0]), sum)
	}
}

// API.md E18 (DECISIONS 463, 469): the layers checked are those the checked packages and their
// imports declare, minus the active ones, in name order; a layer runs over its declarers' importers.
func TestInactiveLayers(t *testing.T) {
	units := layersAnalysis(t).Units()
	cases := []struct {
		pkgs, active, want []string
	}{
		{[]string{"a", "b"}, nil, []string{"live"}},
		{[]string{"a", "b"}, []string{"live"}, nil},
		{[]string{"b"}, nil, []string{"live"}},
		{[]string{"c"}, nil, nil},
	}
	for _, c := range cases {
		if got := inactiveLayers(units, c.pkgs, c.active); !slices.Equal(got, c.want) {
			t.Errorf("inactiveLayers(%v, active %v) = %v, want %v", c.pkgs, c.active, got, c.want)
		}
	}
	over := unitNames(dependents(units, declaring(units, "live")), []string{"a", "b", "c"})
	if !slices.Equal(over, []string{"a", "b"}) {
		t.Errorf("the packages live's analysis covers: %v, want a and its importer b", over)
	}
}

// DECISIONS 469, API.md F7: a merge's file set keeps every id of the run's own set, so a finding
// of that set the merge never held is located in the merged result as in its own.
func TestMergedFilesKeepOwnSet(t *testing.T) {
	base := layersAnalysis(t)
	m := newMerge(base, (*Analysis).resultWhole)
	m.added[layeredKey{}] = true // as though a layer added one
	res, err := m.result(context.Background(), base.Result(), "")
	if err != nil {
		t.Fatal(err)
	}
	last := lastFile(base.r.s.set)
	if last == 0 || res.Files.Path(last) != base.Files().Path(last) {
		t.Errorf("file %d: %q in the merge, want %q", last, res.Files.Path(last), base.Files().Path(last))
	}
}

// DECISIONS 469, API.md X2: Merged skips an analysis that stacks no layer, which names no
// inactive layer, never panicking; with nothing to add the result is the run's own.
func TestMergedSkipsNoLayer(t *testing.T) {
	base := layersAnalysis(t)
	res, err := base.Merged(context.Background(), base.Result(), []*Analysis{layersAnalysis(t)}, "")
	if err != nil || res != base.Result() {
		t.Errorf("Merged of an analysis with no layer: %v, %v, want the run's own result", res, err)
	}
}
