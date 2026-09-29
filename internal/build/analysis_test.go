package build_test

import (
	"context"
	"reflect"
	"sync"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// IMPLEMENTATION-PLAN.md §7.5: two Analyze calls over the same snapshot agree.
func TestAnalyzeIsDeterministic(t *testing.T) {
	for _, sel := range [][]string{{"teamboard"}, {"pipeline"}} {
		p := openExamples(t, t.TempDir())
		a1, err := p.Analyze(context.Background(), sel)
		if err != nil {
			t.Fatalf("%v: %v", sel, err)
		}
		a2, err := p.Analyze(context.Background(), sel)
		if err != nil {
			t.Fatalf("%v: %v", sel, err)
		}
		if render(t, a1.Result().Findings) != render(t, a2.Result().Findings) || a1.Result().Revision != a2.Result().Revision {
			t.Errorf("%v: %s\n%s", sel, render(t, a1.Result().Findings), render(t, a2.Result().Findings))
		}
	}
}

// VIEWMODEL.md J15: Bag is one package's own findings, never merged or truncated with another's.
func TestAnalysisBagIsOnePackage(t *testing.T) {
	fsys := mapFS{
		"law/project.canon": file("project acme {\n  canon: \"0.1\"\n}\n"),
		"law/a/a.canon":     file("/// A.\npackage a\n\n/// X.\nlet x: Int = y\n"),
		"law/b/b.canon":     file("/// B.\npackage b\n\n/// Y.\nlet y: Int = 1\n"),
	}
	p, err := build.Open(fsys, "/law", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	a, err := p.Analyze(context.Background(), []string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	var want []diag.Finding
	for _, f := range a.Result().List {
		if f.Package == "a" {
			want = append(want, f)
		}
	}
	bag := a.Bag("a")
	if bag == nil || len(want) == 0 || !reflect.DeepEqual(bag.Findings(), want) {
		t.Errorf("bag %v, result's a findings %v", bag, want)
	}
	if clean := a.Bag("b"); clean == nil || len(clean.Findings()) != 0 {
		t.Errorf("b bag: %v", clean)
	}
	if a.Files() == nil {
		t.Error("no Files")
	}
}

// VIEWMODEL.md 12.3 `asset`: Layout resolves an unrooted path from its file's directory to its display path.
func TestAnalysisLayout(t *testing.T) {
	fsys := mapFS{
		"law/project.canon": file("project acme {\n  canon: \"0.1\"\n}\n"),
		"law/a/a.canon":     file("/// A.\npackage a\n"),
	}
	p, err := build.Open(fsys, "/law", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	a, err := p.Analyze(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := a.Layout().Resolve("icons", "a", source.Span{}, diag.NewBag(nil, ""))
	if !ok || got.Display != "a/icons" || a.Layout().Dir != "/law" {
		t.Errorf("Layout: %+v %v, dir %q", got, ok, a.Layout().Dir)
	}
}

// VIEWMODEL.md J15: Bag is nil for a package Analyze did not select, loaded as an import or not.
func TestAnalysisBagNilUnlessSelected(t *testing.T) {
	fsys := mapFS{
		"law/project.canon": file("project acme {\n  canon: \"0.1\"\n}\n"),
		"law/a/a.canon":     file("/// A.\npackage a\n\nimport b\n\n/// X.\nlet x: Int = 1\n"),
		"law/b/b.canon":     file("/// B.\npackage b\n\n/// Y.\nlet y: Int = 1\n"),
	}
	p, err := build.Open(fsys, "/law", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	a, err := p.Analyze(context.Background(), []string{"a"})
	if err != nil {
		t.Fatal(err)
	}
	if bag := a.Bag("b"); bag != nil {
		t.Errorf("Bag(\"b\") = %v, want nil: b was loaded, not selected", bag)
	}
}

// fieldValue is rec's field named name (TYPES.md §4.2).
func fieldValue(t *testing.T, rec *value.Record, name string) value.Value {
	t.Helper()
	rt, ok := rec.T.Base().(*types.RecordType)
	if !ok {
		t.Fatalf("rec.T is %T, want *types.RecordType", rec.T)
	}
	for _, f := range rt.Fields {
		if f.Name == name {
			return rec.Fields[f.Index]
		}
	}
	t.Fatalf("no field %q on %v", name, rec.T)
	return nil
}

// CLI.md §3.7: Force and History read one Analysis, port 9000 replacing the 8765 default.
func TestAnalysisForceAndHistory(t *testing.T) {
	p := openExamplesLayered(t, t.TempDir(), []string{"louis"})
	a, err := p.Analyze(context.Background(), []string{"service.resourcestudio"})
	if err != nil {
		t.Fatal(err)
	}
	if errs := a.Result().Summary.Errors; errs != 0 {
		t.Fatalf("%d errors: %s", errs, render(t, a.Result().Findings))
	}
	root := eval.Root{Pkg: "service.resourcestudio", Name: "config"}
	cfg, ok := a.Force(root)
	if !ok {
		t.Fatal("config did not force")
	}
	server := fieldValue(t, cfg.(*value.Record), "server")
	port := fieldValue(t, server.(*value.Record), "port")
	if got, ok := port.(*value.Int); !ok || got.V != 9000 {
		t.Fatalf("port = %v, want 9000", port)
	}
	history := a.History(cfg, server, port)
	if len(history) != 2 {
		t.Fatalf("history has %d values, want 2: %v", len(history), history)
	}
	if got := history[0].(*value.Int).V; got != 9000 {
		t.Errorf("history[0] = %d, want 9000 (the layer's value)", got)
	}
	if got := history[1].(*value.Int).V; got != 8765 {
		t.Errorf("history[1] = %d, want 8765 (the default it replaced)", got)
	}
}

// API.md §3.1, R4: an Analysis is a frozen snapshot, so Force never evaluates an unselected root.
func TestAnalysisFrozenAfterAnalyze(t *testing.T) {
	fsys := mapFS{
		"law/project.canon": file("project acme {\n  canon: \"0.1\"\n}\n"),
		"law/a/a.canon":     file("/// A.\npackage a\n\nimport b\n\n/// X.\nlet x: Int = 1\n"),
		"law/b/b.canon": file("/// B.\npackage b\n\n/// Zero.\nlet zero: Int = 0\n\n" +
			"/// Bad.\nlet bad: Int = 7 / zero\n"),
	}
	p, err := build.Open(fsys, "/law", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	a, err := p.Analyze(context.Background(), []string{"a"})
	if err != nil {
		t.Fatal(err)
	}
	before := render(t, a.Result().Findings)
	if a.Result().Summary.Errors != 0 {
		t.Fatalf("errors before Force: %s", before)
	}
	if _, ok := a.Force(eval.Root{Pkg: "b", Name: "bad"}); ok {
		t.Error("Force forced an unselected package's root")
	}
	if got := render(t, a.Result().Findings); got != before {
		t.Errorf("Result changed after Force:\nbefore %s\nafter  %s", before, got)
	}
	if bag := a.Bag("a"); bag == nil || len(bag.Findings()) != 0 {
		t.Errorf("a's bag changed: %v", bag)
	}
	both, err := p.Analyze(context.Background(), []string{"a", "b"})
	if err != nil {
		t.Fatal(err)
	}
	if both.Result().Summary.Errors == 0 {
		t.Error("selecting b should force bad and report its division by zero")
	}
}

// API.md S7, S8: any number of goroutines may read one Analysis at once.
func TestAnalysisConcurrentReads(t *testing.T) {
	p := openExamplesLayered(t, t.TempDir(), []string{"louis"})
	a, err := p.Analyze(context.Background(), []string{"service.resourcestudio"})
	if err != nil {
		t.Fatal(err)
	}
	root := eval.Root{Pkg: "service.resourcestudio", Name: "config"}
	cfg, ok := a.Force(root)
	if !ok {
		t.Fatal("config did not force")
	}
	unselected := eval.Root{Pkg: "not.selected", Name: "x"}
	const readers = 8
	var wg sync.WaitGroup
	wg.Add(readers)
	for i := 0; i < readers; i++ {
		go func() {
			defer wg.Done()
			a.Force(root)
			a.Force(unselected)
			a.History(cfg)
			a.Result()
			a.Bag("service.resourcestudio").Findings()
			a.Program()
		}()
	}
	wg.Wait()
}

// EVALUATION.md §2.1, §12.2: budget exhaustion is final, so Force refuses a root stage A never reached, exactly like an unselected one.
func TestAnalysisFrozenAfterBudgetExhaustion(t *testing.T) {
	fsys := mapFS{
		"law/project.canon": file("project acme {\n  canon: \"0.1\"\n  budget: 200\n}\n"),
		"law/a/a.canon": file("/// A.\npackage a\n\n" +
			"local fn loop(n: Int) -> Int {\n  var i = 0\n  while i < n { i += 1 }\n  return i\n}\n\n" +
			"/// Big.\nlocal let big: Int = loop(1000)\n\n" +
			"/// Later.\nlocal let later: Int = 1 / zero\n\n" +
			"/// Zero.\nlocal let zero: Int = 0\n"),
	}
	p, err := build.Open(fsys, "/law", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	a, err := p.Analyze(context.Background(), []string{"a"})
	if err != nil {
		t.Fatal(err)
	}
	if n := a.Result().Summary.Errors; n != 1 {
		t.Fatalf("errors = %d, want 1 (the exhausted budget's finding): %s", n, render(t, a.Result().Findings))
	}
	bag := a.Bag("a")
	if bag == nil {
		t.Fatal("no bag for a")
	}
	wantFindings, before := bag.Findings(), render(t, a.Result().Findings)
	for _, name := range []string{"big", "later", "zero"} {
		if v, ok := a.Force(eval.Root{Pkg: "a", Name: name}); ok || v != nil {
			t.Errorf("Force(%s) = %v, %t, want nil, false", name, v, ok)
		}
	}
	if got := render(t, a.Result().Findings); got != before {
		t.Errorf("Result changed after Force:\nbefore %s\nafter  %s", before, got)
	}
	if !reflect.DeepEqual(bag.Findings(), wantFindings) {
		t.Errorf("Bag(\"a\") changed after Force: before %v, after %v", wantFindings, bag.Findings())
	}
}

// EVALUATION.md §2.1, API.md R4: Force reads only what stage A settled, and reports nothing.
func TestAnalysisForceReadsSettled(t *testing.T) {
	fsys := mapFS{
		"law/project.canon": file("project acme {\n  canon: \"0.1\"\n}\n"),
		"law/a/a.canon": file("/// A.\npackage a\n\n/// Good.\nlet good: Int = 6 * 7\n\n/// Bad.\nlet bad: Int = [1][3]\n\n" +
			"/// Twice.\nfn twice(n: Int) -> Int {\n  return n * 2\n}\n"),
	}
	p, err := build.Open(fsys, "/law", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	a, err := p.Analyze(context.Background(), []string{"a"})
	if err != nil {
		t.Fatal(err)
	}
	before := render(t, a.Result().Findings)
	if v, ok := a.Force(eval.Root{Pkg: "a", Name: "good"}); !ok || v.CanonText() != "42" {
		t.Errorf("Force(good) = %v, %t, want 42", v, ok)
	}
	for _, name := range []string{"bad", "twice", "nothing"} {
		if v, ok := a.Force(eval.Root{Pkg: "a", Name: name}); ok || v != nil {
			t.Errorf("Force(%s) = %v, %t, want nil, false", name, v, ok)
		}
	}
	if got := render(t, a.Result().Findings); got != before {
		t.Errorf("Result changed after Force:\nbefore %s\nafter  %s", before, got)
	}
}

// API.md R6, EVALUATION.md §7.2: Cause is a poisoned root's own error or its upstream root's.
func TestAnalysisCause(t *testing.T) {
	fsys := mapFS{
		"law/project.canon": file("project acme {\n  canon: \"0.1\"\n}\n"),
		"law/a/a.canon":     file("/// A.\npackage a\n\nimport b\n\n/// Reads b.\nlet total: Int = b.bad + 1\n\n/// Fine.\nlet fine: Int = 1\n"),
		"law/b/b.canon":     file("/// B.\npackage b\n\n/// Bad.\nlet bad: Int = [1, 2][5]\n"),
	}
	p, err := build.Open(fsys, "/law", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	a, err := p.Analyze(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, root := range []eval.Root{{Pkg: "a", Name: "total"}, {Pkg: "b", Name: "bad"}} {
		if c := a.Cause(root); len(c) != 1 || c[0].Package != "b" {
			t.Errorf("Cause(%v) = %v, want b's one error", root, c)
		}
	}
	if c := a.Cause(eval.Root{Pkg: "a", Name: "fine"}); len(c) != 0 {
		t.Errorf("Cause(fine) = %v", c)
	}
}
