package build_test

import (
	"context"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
)

// driversSyntax is a dependent of a whose table a.Target drives reads a function holding a
// syntax error: an assignment with no target; a clean table of the same package sits beside it.
const driversSyntax = `/// C.
package c

import a

/// Loops.
local fn loop(n: Int) -> Int {
  var i = 0
  while 0 < n {
    = 0
  }
  return i
}

/// Own kinds.
let own: table a.Kind = {
  c1 { goal: visit, n: loop(3) }
}

/// Clean kinds, beside the flawed table.
let fine: table a.Kind = {
  c2 { goal: visit, n: 4 }
}

/// A use of own.
record Use {
  /// Its kind.
  kind: ref own
  /// Its target.
  target: a.Target(kind)?
}

/// A use of fine.
record UseFine {
  /// Its kind.
  kind: ref fine
  /// Its target.
  target: a.Target(kind)?
}
`

// TYPES.md §1, EVALUATION.md §2.1: a syntax error breaks its declaration, not the clean table beside it.
func TestDriversSkipSyntaxErrors(t *testing.T) {
	for _, limit := range []int{0, 1} {
		fsys := driversFS()
		fsys["p/c/c.canon"] = file(driversSyntax)
		p, err := build.Open(fsys, "/p", build.Options{MaxFindings: limit})
		if err != nil {
			t.Fatal(err)
		}
		driversSkipped(t, p)
	}
}

// driversSkipped builds and analyzes a alone over driversSyntax.
func driversSkipped(t *testing.T, p *build.Project) {
	t.Helper()
	res, err := p.Build(context.Background(), build.BuildOptions{Packages: []string{"a"}, Targets: viewOnly})
	if err != nil {
		t.Fatalf("build a: %v", err)
	}
	if res.Summary.Errors != 0 {
		t.Errorf("build a reports %v", codes(res.List))
	}
	got := decodeModel(t, viewOutput(t, res, "@out/a.view.json").Content).Types["a.Target"].Drivers
	if _, ok := got["c:own"]; ok || got["a:kinds"] == nil {
		t.Errorf("drivers %v, want a:kinds and no c:own", got)
	}
	if got["c:fine"]["c2"] != "visit" {
		t.Errorf("drivers %v, want c:fine to hold c2 visit", got)
	}
	an, err := p.Analyze(context.Background(), []string{"a"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := an.ViewModel(context.Background(), "a"); err != nil {
		t.Errorf("Analysis.ViewModel(a): %v", err)
	}
}
