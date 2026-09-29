package build_test

import (
	"bytes"
	"context"
	"errors"
	"maps"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	viewgen "github.com/fantasim/canonlang/internal/gen/view"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/syntax"
)

const (
	driversA = `/// A.
package a

/// A goal.
enum Goal { kill, visit }

/// A kind.
record Kind {
  /// Its goal.
  goal: Goal
  /// Its weight.
  n: Int
}

/// Kinds.
let kinds: table Kind = {
  k1 { goal: kill, n: 1 }
  k2 { goal: visit, n: 2 }
}

/// What a kind targets.
type Target(k: Kind) = match k.goal {
  kill => String
  visit => Int
}

emit view { out: "@out/a.view.json" }
`
	driversB = `/// B.
package b

import a

/// A step.
record Step {
  /// Its kind.
  kind: ref a.kinds
  /// Its target.
  target: a.Target(kind)?
}

/// Steps.
let steps: [Step] = []
`
	// c applies a.Target to its own tables, has an error, and one table fails to evaluate.
	driversC = `/// C.
package c

import a

/// Own kinds.
let own: table a.Kind = {
  c1 { goal: visit, n: 3 }
}

/// Failing kinds.
let failing: table a.Kind = {
  f1 { goal: kill, n: 1 / 0 }
}

/// A use of own.
record Use {
  /// Its kind.
  kind: ref own
  /// Its target.
  target: a.Target(kind)?
}

/// A use of failing.
record Fail {
  /// Its kind.
  kind: ref failing
  /// Its target.
  target: a.Target(kind)?
}

/// Broken.
let broken: Int = "one"
`
)

// driversFS is a project whose package a declares a type function b and c apply to a's table
// and c's own; c has a static error and a table that fails to evaluate.
func driversFS() mapFS {
	return mapFS{
		"p/project.canon": file(viewProject), "p/a/a.canon": file(driversA),
		"p/b/b.canon": file(driversB), "p/c/c.canon": file(driversC),
	}
}

// VIEWMODEL.md 12.3 (typeFunction drivers), J5, API.md R9, EVALUATION.md 2.1: a's model is
// the same bytes whatever the build selects; the packages read only for its drivers report
// nothing, block nothing, and a table that fails contributes no entries.
func TestDriversCoverTheProject(t *testing.T) {
	var models [][]byte
	for _, sel := range [][]string{{"a"}, {"a", "b"}, nil} {
		res := buildTree(t, driversFS(), build.BuildOptions{Packages: sel, Targets: viewOnly})
		if sel != nil && slices.ContainsFunc(res.List, func(f diag.Finding) bool { return f.Package != "a" && f.Package != "" }) {
			t.Errorf("build %v reports another package's findings: %+v", sel, res.List)
		}
		if sel != nil && res.Summary.Errors != 0 {
			t.Errorf("build %v: errors %v", sel, codes(res.List))
		}
		models = append(models, viewOutput(t, res, "@out/a.view.json").Content)
	}
	for i, m := range models[1:] {
		if !bytes.Equal(m, models[0]) {
			t.Errorf("a's model, selection %d, differs from a alone (%d vs %d bytes)", i+1, len(m), len(models[0]))
		}
	}
	got := decodeModel(t, models[0]).Types["a.Target"].Drivers
	want := map[string]map[string]string{"a:kinds": {"k1": "kill", "k2": "visit"}, "c:own": {"c1": "visit"}}
	if !maps.EqualFunc(got, want, maps.Equal) {
		t.Errorf("a.Target drivers = %v, want %v", got, want)
	}
	p, err := build.Open(driversFS(), "/p", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	an, err := p.Analyze(context.Background(), []string{"a"})
	if err != nil {
		t.Fatal(err)
	}
	m, err := an.ViewModel(context.Background(), "a")
	if err != nil {
		t.Fatal(err)
	}
	if b, err := viewgen.Write(m); err != nil || !bytes.Equal(b, models[0]) {
		t.Errorf("Analysis.ViewModel(a) differs from the emit view file: %v", err)
	}
}

// driversLoad is c with a table read by an unsupported load (a text file) that a.Target drives.
const driversLoad = `
/// Loaded.
let loaded: table a.Kind = load.dir("x.txt")

/// A use of loaded.
record Load {
  /// Its kind.
  kind: ref loaded
  /// Its target.
  target: a.Target(kind)?
}
`

// EVALUATION.md 2.1, VIEWMODEL.md 12.3, J5: an unsupported load met while reading a value only
// for drivers contributes no entries, in the drivers-only program ({a}, {a,b}) and in the run's
// own ({a,b,e}: e loads c), with the same bytes; the same load forced by the build is ErrLoad.
func TestDriversIgnoreUnsupportedLoads(t *testing.T) {
	fsys := driversFS()
	fsys["p/c/c.canon"] = file(driversC + driversLoad)
	fsys["p/c/x.txt"] = file("1")
	fsys["p/e/e.canon"] = file("/// E.\npackage e\n\nimport c\n")
	var models [][]byte
	for _, sel := range [][]string{{"a"}, {"a", "b"}, {"a", "b", "e"}} {
		res := buildTree(t, fsys, build.BuildOptions{Packages: sel, Targets: viewOnly})
		models = append(models, viewOutput(t, res, "@out/a.view.json").Content)
		if !bytes.Equal(models[len(models)-1], models[0]) {
			t.Errorf("build %v: a's model differs from a alone", sel)
		}
	}
	got := decodeModel(t, models[0]).Types["a.Target"].Drivers
	if _, ok := got["c:loaded"]; ok || got["c:own"] == nil || got["a:kinds"] == nil {
		t.Errorf("drivers %v, want a:kinds and c:own, no c:loaded", got)
	}
	p, err := build.Open(fsys, "/p", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Build(context.Background(), build.BuildOptions{Targets: viewOnly}); !errors.Is(err, build.ErrLoad) {
		t.Errorf("building c: %v, want ErrLoad (the load is unsupported)", err)
	}
}

// API.md S11, VIEWMODEL.md J5: a call cancelled while the drivers-only program is checked
// returns the context's error and keeps nothing; a later call writes the same bytes.
func TestDriversCancelled(t *testing.T) {
	const (
		wideCheck = 2 // Analyze's check, then the drivers' program's
		checks    = 3 // and the drivers' program's again, after the cancelled call
	)
	want := viewOutput(t, buildTree(t, driversFS(), build.BuildOptions{Packages: []string{"a"}, Targets: viewOnly}), "@out/a.view.json").Content
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	checker := func(c context.Context, proj *project.Project, files []*syntax.File, bags map[string]*diag.Bag) *check.Program {
		if calls++; calls == wideCheck {
			cancel()
		}
		return check.Check(c, proj, files, bags, eval.NewFolder(bags, eval.Options{}))
	}
	p, err := build.Open(driversFS(), "/p", build.Options{Checker: checker})
	if err != nil {
		t.Fatal(err)
	}
	a, err := p.Analyze(ctx, []string{"a"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.ViewModel(ctx, "a"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled ViewModel: %v, want context.Canceled", err)
	}
	m, err := a.ViewModel(context.Background(), "a")
	if err != nil {
		t.Fatal(err)
	}
	b, err := viewgen.Write(m)
	if err != nil || !bytes.Equal(b, want) || calls != checks {
		t.Errorf("after a cancelled call: %v, same bytes %t, checks %d", err, bytes.Equal(b, want), calls)
	}
}
