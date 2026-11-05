package workspace

import (
	"context"
	"errors"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/fantasim/canonlang/internal/build"
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

// DECISIONS 469, API.md R3a: a check with no inactive layer, or whose layers add nothing, is the
// kept analysis's own result, the same value; a run's own analysis is not repeated.
func TestCheckNothingNewKept(t *testing.T) {
	for _, active := range [][]string{nil, {"live"}} {
		s := selectionOn(t, active...)
		a, err := Analyze(context.Background(), s, nil)
		if err != nil {
			t.Fatal(err)
		}
		res, err := Check(context.Background(), s, nil, "")
		if err != nil || res != a.Result() {
			t.Errorf("active %v: %v, %v: want the analysis's own result", active, res, err)
		}
	}
}

// DECISIONS 469: a check's layer analyses stop on a cancelled context, between layers, with its
// error, the check's own result kept.
func TestCheckLayersHonourContext(t *testing.T) {
	s := selectionOn(t)
	a, err := Analyze(context.Background(), s, nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if res, _, err := a.Layered(ctx, a.Result(), build.LayerRun{Pkgs: a.Selected()}); !errors.Is(err, context.Canceled) || res != a.Result() {
		t.Errorf("the layers on a cancelled context: %v, want context.Canceled and the check's own result", err)
	}
}

// DECISIONS 469, API.md W13: an edit event reads the re-check's merged findings kept beside its
// analysis, analysing nothing again; any other analysis of the snapshot is merged afresh.
func TestCheckedKeptForTheEdit(t *testing.T) {
	ctx := context.Background()
	s := selectionOn(t)
	edited, err := AnalyzeOnly(ctx, s, []string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	kept := &build.Result{}
	s.kept.event, s.kept.eventOf = kept, edited
	if res, err := Checked(ctx, s, edited); err != nil || res != kept {
		t.Errorf("Checked of the edit's analysis: %v, %v, want the kept result", res, err)
	}
	other, err := Analyze(ctx, s, nil)
	if err != nil {
		t.Fatal(err)
	}
	if res, err := Checked(ctx, s, other); err != nil || res == kept || res.Summary.Errors != 0 {
		t.Errorf("Checked of another analysis: %v, %v, want its own clean merge", res, err)
	}
}
