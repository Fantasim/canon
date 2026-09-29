package build_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/api/vm"
	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
	viewgen "github.com/fantasim/canonlang/internal/gen/view"
	"github.com/fantasim/canonlang/internal/ir"
)

var viewOnly = []ir.Target{ir.TargetView}

const (
	viewProject = "project acme {\n  canon: \"0.1\"\n  roots {\n    out: \"out\"\n  }\n  languages: [en, fr]\n}\n"
	viewB       = "/// B.\npackage b\n\n/// A thing.\nrecord Thing {\n  /// Its name.\n  name: String\n}\n\n/// Things.\nlet things: table Thing = {\n  one { name: \"One\" }\n  two { name: \"Two\" }\n}\n\n/// Read by a's view only.\nlet label: String = \"B\"\n\n/// Read by nothing.\nlet unread: Int = 1 / 0\n"
	viewA       = `/// A.
package a

import b

/// A use.
record Use {
  /// How many.
  count: Int

  warn big: count < 10 else "count {count} is big"

  /// Its label.
  fn label(self) -> String { return "N{count}" }
}

/// A pick.
record Pick {
  /// Its thing.
  thing: ref b.things
}

/// Uses.
let uses: table Use = {
  first { count: 20 }
}

/// Picks.
let picks: [Pick] = []

view Use {
  title "{id}: {label()}"
  subtitle "{b.label}"
}

emit view { out: "@out/a.view.json" }
`
	viewFr = "package a\ntranslation fr\n\nUse.check.big \"compte {count} trop grand\"\n"
	viewC  = `/// C.
package c

/// A thing.
record Thing {
  /// Its size.
  size: Int
}

/// Things.
let things: table Thing = {
  one { size: 1 }
}

/// Broken at evaluation.
let bad: Int = 1 / 0

view Thing {
  title "{id}"
}

emit view { out: "@out/c.view.json" }
emit json { out: "@out/c.json", values: [things] }
`
)

// viewFS is a project whose package a reads b's table only through a ref type, and translates
// a named check (VIEWMODEL.md C3, J15).
func viewFS() mapFS {
	return mapFS{"p/project.canon": file(viewProject), "p/a/a.canon": file(viewA), "p/a/a.fr.canon": file(viewFr), "p/b/b.canon": file(viewB)}
}

// viewOutput is the one output of res whose display path is display, failing without it.
func viewOutput(t *testing.T, res *build.BuildResult, display string) build.Output {
	t.Helper()
	for _, o := range res.Outputs {
		if o.Path == display {
			return o
		}
	}
	t.Fatalf("no output %s in %+v", display, res.Outputs)
	return build.Output{}
}

// decodeModel is a view model's bytes read back into its structs.
func decodeModel(t *testing.T, b []byte) *vm.ViewModel {
	t.Helper()
	var m vm.ViewModel
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return &m
}

// titles are the rendered titles of an index's rows, by key.
func titles(idx vm.SearchIndex) map[string]string {
	out := map[string]string{}
	for _, r := range idx.Rows {
		if r.Title != nil {
			out[r.Key.Text] = *r.Title
		}
	}
	return out
}

// VIEWMODEL.md §15.1, J1, J2, J15, CODEGEN.md §2.1, API.md R9: the emit view out is the golden's bytes.
func TestPipelineViewModel(t *testing.T) {
	p := openExamples(t, t.TempDir())
	res, err := p.Build(context.Background(), build.BuildOptions{Packages: []string{"pipeline"}, Targets: viewOnly, Check: true})
	if err != nil {
		t.Fatal(err)
	}
	o := viewOutput(t, res, "pipeline/out/potion.view.json")
	want, err := os.ReadFile(filepath.Join(examplesDir, "pipeline", "expected", "potion.view.json"))
	if err != nil {
		t.Fatal(err)
	}
	if o.Target != ir.TargetView || o.Package != "pipeline" || o.Status != build.StatusStale {
		t.Errorf("output %s: target %v, package %s, status %v", o.Path, o.Target, o.Package, o.Status)
	}
	if !bytes.Equal(o.Content, want) {
		t.Errorf("pipeline view model:\n%s\nwant\n%s", o.Content, want)
	}
	a, err := p.Analyze(context.Background(), []string{"pipeline"})
	if err != nil {
		t.Fatal(err)
	}
	m, err := a.ViewModel(context.Background(), "pipeline")
	if err != nil {
		t.Fatal(err)
	}
	if b, err := viewgen.Write(m); err != nil || !bytes.Equal(b, want) {
		t.Errorf("Analysis.ViewModel differs from the emit view file: %v", err)
	}
	if _, err := a.ViewModel(context.Background(), "studio"); !errors.Is(err, build.ErrNotSelected) {
		t.Errorf("a package not selected: %v", err)
	}
}

// VIEWMODEL.md C3, J12, J5, EVALUATION.md §2.1: what a model reads is evaluated on demand; b.unread stays silent.
func TestViewModelSettlesReferredPackages(t *testing.T) {
	alone := buildTree(t, viewFS(), build.BuildOptions{Packages: []string{"a"}, Targets: viewOnly})
	if alone.Summary.Errors != 0 {
		t.Errorf("building a reports b's unread value: %v", codes(alone.List))
	}
	m := decodeModel(t, viewOutput(t, alone, "@out/a.view.json").Content)
	ref := m.Types["a.Pick"].Fields[0].Type
	if ref.Count == nil || ref.Active == nil || *ref.Count != 2 || *ref.Active != 2 {
		t.Errorf("ref b.things counts %v/%v, want 2/2", ref.Count, ref.Active)
	}
	if rows := m.Search["a:uses"].Rows; len(rows) != 1 || rows[0].Subtitle == nil || *rows[0].Subtitle != "B" {
		t.Errorf("a subtitle reading b.label, evaluated on demand: %+v", rows)
	}
	both := buildTree(t, viewFS(), build.BuildOptions{Packages: []string{"a", "b"}, Targets: viewOnly})
	if a, b := viewOutput(t, alone, "@out/a.view.json"), viewOutput(t, both, "@out/a.view.json"); !bytes.Equal(a.Content, b.Content) {
		t.Error("a's model depends on the selection")
	}
}

// VIEWMODEL.md §3.4, §9.2 X4, S2: a title renders the magic name id and a method of the target.
func TestViewModelRendersTemplates(t *testing.T) {
	res := buildTree(t, viewFS(), build.BuildOptions{Packages: []string{"a"}, Targets: viewOnly})
	m := decodeModel(t, viewOutput(t, res, "@out/a.view.json").Content)
	if got := titles(m.Search["a:uses"]); !maps.Equal(got, map[string]string{"first": "first: N20"}) {
		t.Errorf("titles %v", got)
	}
}

// VIEWMODEL.md §3.4, S2: `{name()}` titles and `search { name(), id }` terms of resource.vocab, game.items.
func TestExampleTitles(t *testing.T) {
	p := openExamples(t, t.TempDir())
	a, err := p.Analyze(context.Background(), []string{"resource.vocab", "game.items"})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ pkg, index, key, title string }{
		{"resource.vocab", "resource.vocab:items", "II_GEN_MAT_MOONSTONE", "Moonstone"},
		{"resource.vocab", "resource.vocab:eventTypes", "COMBAT_KILL_BOSS", "Boss Kill"},
		{"game.items", "game.items:items", "II_WEA_AXE_ANGEL", "Angel Axe"},
	} {
		m, err := a.ViewModel(context.Background(), c.pkg)
		if err != nil {
			t.Fatal(err)
		}
		idx := m.Search[c.index]
		if got := titles(idx)[c.key]; got != c.title {
			t.Errorf("%s %s: title %q, want %q (indexes %v)", c.index, c.key, got, c.title, slices.Sorted(maps.Keys(m.Search)))
		}
		if i := slices.IndexFunc(idx.Rows, func(r vm.Row) bool { return r.Key.Text == c.key }); i < 0 || !slices.Contains(idx.Rows[i].Terms, c.title) {
			t.Errorf("%s %s: terms lack %q", c.index, c.key, c.title)
		}
	}
}

// VIEWMODEL.md J1, J5, IMPLEMENTATION-PLAN §7.5: every example's emit view is written, alike on two builds.
func TestExampleViewModels(t *testing.T) {
	var runs [2]*build.BuildResult
	for i := range runs {
		res, err := openExamples(t, t.TempDir()).Build(context.Background(), build.BuildOptions{Targets: viewOnly, Check: true})
		if err != nil {
			t.Fatal(err)
		}
		runs[i] = res
	}
	const emitViews = 9 // the examples' `emit view` declarations
	if len(runs[0].Outputs) != emitViews || len(runs[1].Outputs) != emitViews {
		t.Fatalf("%d and %d view models, want %d", len(runs[0].Outputs), len(runs[1].Outputs), emitViews)
	}
	for i, o := range runs[0].Outputs {
		if q := runs[1].Outputs[i]; o.Path != q.Path || !bytes.Equal(o.Content, q.Content) || !json.Valid(o.Content) {
			t.Errorf("%s differs between two builds, or is no JSON", o.Path)
		}
	}
}

// VIEWMODEL.md J15, I18N.md B5, T4: a named check's finding carries its translated messages.
func TestViewModelTranslatedMessages(t *testing.T) {
	res := buildTree(t, viewFS(), build.BuildOptions{Packages: []string{"a"}, Targets: viewOnly})
	m := decodeModel(t, viewOutput(t, res, "@out/a.view.json").Content)
	i := slices.IndexFunc(m.Findings, func(f vm.Finding) bool { return f.Check == "big" })
	if i < 0 {
		t.Fatalf("no finding of check big in %+v", m.Findings)
	}
	f := m.Findings[i]
	if f.Message != "count 20 is big" || !maps.Equal(f.Messages, map[string]string{"fr": "compte 20 trop grand"}) {
		t.Errorf("finding %q, messages %v", f.Message, f.Messages)
	}
	for _, g := range m.Findings {
		if g.Check == "" && g.Messages != nil {
			t.Errorf("%s has messages %v", g.Code, g.Messages)
		}
	}
}

// VIEWMODEL.md J15, EVALUATION.md §8.3: a check `at` a field (resource.farm's unreachable_levels) finds its instance.
func TestFarmMessages(t *testing.T) {
	a, err := openExamples(t, t.TempDir()).Analyze(context.Background(), []string{"resource.farm"})
	if err != nil {
		t.Fatal(err)
	}
	m, err := a.ViewModel(context.Background(), "resource.farm")
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(m.Findings, func(f vm.Finding) bool { return f.Check == "unreachable_levels" })
	want := "maxLevel vaut 3 mais seuls 2 paliers existent : le niveau 3 et au-delà sont inatteignables (GetLevel renvoie nullptr)"
	if i < 0 || m.Findings[i].Messages["fr"] != want {
		t.Errorf("findings %+v", m.Findings)
	}
}

// VIEWMODEL.md J4, J15, API.md B1, EVALUATION.md §1 phase 8: with an error, only the view model is written.
func TestViewModelWithErrors(t *testing.T) {
	fsys := mapFS{"p/project.canon": file(viewProject), "p/c/c.canon": file(viewC)}
	res := buildTree(t, fsys, build.BuildOptions{})
	if res.Summary.Errors == 0 {
		t.Fatal("no error")
	}
	if len(res.Outputs) != 1 {
		t.Fatalf("outputs %+v, want the view model only", res.Outputs)
	}
	o := viewOutput(t, res, "@out/c.view.json")
	if written := fsys["p/out/c.view.json"]; o.Status != build.StatusWritten || written == nil || !bytes.Equal(written.Data, o.Content) {
		t.Fatalf("the view model was not written: %v", o.Status)
	}
	m := decodeModel(t, o.Content)
	if v, ok := m.Values["c:bad"]; !ok || !v.Failed {
		t.Errorf("c:bad = %+v, want failed", v)
	}
	code := string(diag.E4102.Def().Code)
	if !slices.ContainsFunc(m.Findings, func(f vm.Finding) bool { return f.Code == code && f.Severity == "error" }) {
		t.Errorf("findings %+v lack %s", m.Findings, code)
	}
	if got := titles(m.Search["c:things"]); !maps.Equal(got, map[string]string{"one": "one"}) {
		t.Errorf("titles %v", got)
	}
}
