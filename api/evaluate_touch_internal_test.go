package canon

import (
	"context"
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/workspace"
)

// The examples project as the ≡ test reads it: read roots at their fixtures, written ones unused.
const (
	touchExamples = "../examples"
	touchFixtures = "_fixtures"
	touchElements = 3 // elements of each collection evaluated besides the value itself
)

// touchReadRoots are the roots examples/project.canon reads from, at their fixtures.
var touchReadRoots = []string{"resource", "client"}

// touchWriteRoots are the roots examples/project.canon writes to; an analysis writes nothing.
var touchWriteRoots = []string{"source", "services", "sovcommon", "web", "parity", "generated"}

// API.md V13, V14, E17 (log-2026-09-29 M4 P14-r): within the budget, an evaluation on the analysis
// of the packages it touches gives what every package's gives, for every top-level value of the
// examples and the first elements of each collection.
func TestEvaluateTouchedEqualsAll(t *testing.T) {
	p := openTouchExamples(t)
	ctx := context.Background()
	s, err := p.read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	all, err := analyze(ctx, s, nil)
	if err != nil {
		t.Fatal(err)
	}
	paths := touchPaths(t, p, all.Program())
	if len(paths) == 0 {
		t.Fatal("no value to evaluate")
	}
	for _, path := range paths {
		parsed, err := edit.Parse(path)
		if err != nil {
			t.Fatal(err)
		}
		pkgs, err := workspace.Touching(ctx, s, parsed.Package)
		if err != nil || len(pkgs) == 0 {
			t.Fatalf("%s touches %v: %v", path, pkgs, err)
		}
		touched, err := analyze(ctx, s, pkgs)
		if err != nil {
			t.Fatal(err)
		}
		want := touchJSON(t, p, evalOn{s: s, a: all, lang: p.lang}, path)
		if got := touchJSON(t, p, evalOn{s: s, a: touched, lang: p.lang}, path); got != want {
			t.Errorf("%s: touched packages give\n%s\nevery package gives\n%s", path, got, want)
		}
	}
}

// openTouchExamples opens examples/ on the OS, its read roots at their fixtures.
func openTouchExamples(t *testing.T) *Project {
	t.Helper()
	roots := map[string]string{}
	for _, name := range touchReadRoots {
		roots[name] = filepath.Join(touchFixtures, name)
	}
	out := t.TempDir()
	for _, name := range touchWriteRoots {
		roots[name] = filepath.Join(out, name)
		if err := os.MkdirAll(roots[name], 0o750); err != nil { // SPEC §3.1: a required root exists
			t.Fatal(err)
		}
	}
	p, err := Open(touchExamples, Options{Roots: roots, Cache: "off"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	return p
}

// touchPaths is every top-level let and const of prog, each followed by its first elements.
func touchPaths(t *testing.T, p *Project, prog *check.Program) []string {
	t.Helper()
	var out []string
	for _, cp := range prog.Packages {
		for _, obj := range cp.Decls {
			if obj.Kind() != check.ObjLet && obj.Kind() != check.ObjConst {
				continue
			}
			path := cp.Path + ":" + obj.Name()
			v, err := p.Value(context.Background(), path)
			if err != nil {
				continue // a value Value cannot show, which Evaluate refuses alike
			}
			out = append(out, path)
			for _, c := range v.Children()[:min(touchElements, len(v.Children()))] {
				out = append(out, c.Path)
			}
		}
	}
	slices.Sort(out)
	return slices.Compact(out)
}

// touchJSON is path's evaluation on on, as JSON, failing the test on an error.
func touchJSON(t *testing.T, p *Project, on evalOn, path string) string {
	t.Helper()
	res, err := p.evaluateOn(context.Background(), on, path)
	if err != nil {
		return "error: " + err.Error()
	}
	res.Headings = maps.Clone(res.Headings)
	data, err := json.Marshal(res)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
