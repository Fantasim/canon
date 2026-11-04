package workspace

import (
	"context"
	"errors"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
)

// layeredOn is an outcome whose After is selectionFS with no layer active, re-checked as a and b.
func layeredOn(t *testing.T) *EditOutcome {
	t.Helper()
	return layeredOver(t, selectionFS(), "a", "b")
}

// layeredOver is layeredOn over fsys, re-checked as pkgs.
func layeredOver(t *testing.T, fsys fstest.MapFS, pkgs ...string) *EditOutcome {
	t.Helper()
	b, err := build.Open(&clockFS{MapFS: fsys}, "/law", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	p := New(b)
	t.Cleanup(p.Close)
	s, err := p.Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	a, err := AnalyzeOnly(context.Background(), s, pkgs)
	if err != nil {
		t.Fatal(err)
	}
	o := &EditOutcome{After: s, checked: a}
	if o.Checked, err = a.OwnIn(context.Background(), ""); err != nil {
		t.Fatal(err)
	}
	return o
}

// API.md E18 (DECISIONS 463): the re-check of the layers a session does not apply stops
// on a cancelled context, between layers, with its error.
func TestLayeredHonoursContext(t *testing.T) {
	o := layeredOn(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := o.layered(ctx, o.checked, []string{"a", "b"}, ""); !errors.Is(err, context.Canceled) {
		t.Errorf("layered on a cancelled context: %v, want context.Canceled", err)
	}
}

// API.md E18 (DECISIONS 463): an inactive layer that finds nothing new leaves the re-check's
// result as it was, the same value.
func TestLayeredNothingNew(t *testing.T) {
	o := layeredOn(t)
	before := o.Checked
	if err := o.layered(context.Background(), o.checked, []string{"a", "b"}, ""); err != nil {
		t.Fatal(err)
	}
	if o.Checked != before {
		t.Errorf("layered replaced a result it added nothing to")
	}
}

// API.md S5 (DECISIONS 463): what only an inactive layer's analysis consulted joins the edit's
// staleness check.
func TestLayeredReadsKept(t *testing.T) {
	fsys := selectionFS()
	fsys["law/a/far.layer.canon"] = &fstest.MapFile{Data: []byte("package a\nlayer far\n\namend cfg {\n  port: load(\"p.json\")\n}\n")}
	fsys["law/a/p.json"] = &fstest.MapFile{Data: []byte("4\n")}
	fsys["law/g/g.canon"] = &fstest.MapFile{Data: []byte("/// G.\npackage g\n\n/// An icon.\ntype Icon = asset(\"../icons\", ext: [png])\n\n" +
		"/// A look.\nrecord Look {\n  /// Its icon.\n  icon: Icon?\n}\n\n/// The look.\nlet look: Look = {}\n")}
	fsys["law/g/art.layer.canon"] = &fstest.MapFile{Data: []byte("package g\nlayer art\n\namend look {\n  icon: \"a.png\"\n}\n")}
	fsys["law/icons/a.png"] = &fstest.MapFile{Data: []byte("png")}
	o := layeredOver(t, fsys, "a", "b", "g")
	if err := o.layered(context.Background(), o.checked, []string{"a", "b", "g"}, ""); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"a/p.json", "icons"} {
		if !slices.ContainsFunc(o.layerReads, func(r build.Read) bool { return r.Display == want }) {
			t.Errorf("the layer analyses' reads %v, want %s, which only an inactive layer consults", o.layerReads, want)
		}
	}
	if slices.ContainsFunc(o.checked.Consulted("g"), func(r build.Read) bool { return r.Display == "icons" }) {
		t.Error("the session's own re-check lists the asset root: the case does not pin the layer's")
	}
}

// API.md F7 (DECISIONS 463): the merged findings are cut once as the bags cut them: each package at
// the session's limit, the project's own, package "", at its own bag's.
func TestCutOnceOwnLimit(t *testing.T) {
	o := layeredOn(t)
	own, a := diag.NewBag(o.checked.Files(), ""), diag.NewBag(o.checked.Files(), "a")
	for _, msg := range []string{"one", "two", "three"} {
		diag.E5001.At(source.Span{}, msg).Report(own)
	}
	for _, msg := range []string{"four", "five"} {
		diag.W5001.At(source.Span{}, msg).Report(a)
	}
	m := &merge{parts: []*build.Analysis{o.checked}}
	for _, f := range append(own.Findings(), a.Findings()...) {
		m.entries = append(m.entries, merged{f: f})
	}
	kept, sum := m.cut(1, 2)
	want := []diag.Truncation{{Package: "", Errors: 1}, {Package: "a", Warnings: 1}}
	if len(kept[0]) != 3 || sum.Errors != 3 || sum.Warnings != 2 || !slices.Equal(sum.Truncated, want) {
		t.Errorf("kept %d, summary %+v: want two of the project's and one of a's kept, the rest counted", len(kept[0]), sum)
	}
}

// API.md E18 (DECISIONS 463): the layers checked are those the re-checked packages and their
// imports declare, minus the active ones, in name order; a layer runs over its declarers' importers.
func TestInactiveLayers(t *testing.T) {
	o := layeredOn(t)
	units := o.checked.Units()
	cases := []struct {
		pkgs, active, want []string
	}{
		{[]string{"a", "b"}, nil, []string{"live"}},
		{[]string{"a", "b"}, []string{"live"}, nil},
		{[]string{"b"}, nil, []string{"live"}},
	}
	for _, c := range cases {
		if got := inactiveLayers(units, importedBy(units, c.pkgs), c.active); !slices.Equal(got, c.want) {
			t.Errorf("inactiveLayers(%v, active %v) = %v, want %v", c.pkgs, c.active, got, c.want)
		}
	}
	if got := importing(units, declaring(units, "live")...); !slices.Equal(got, []string{"a", "b"}) {
		t.Errorf("the packages live's analysis covers: %v, want a and its importer b", got)
	}
}
