package build_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
)

const (
	foldProject = "project acme {\n  canon: \"0.1\"\n  budget: %d\n  roots {\n    out: \"out\"\n  }\n  go_module {\n    out: \"example.com/out\"\n  }\n}\n"
	foldSource  = `/// A.
package a

emit go { out: "@out/g", package: "g", mode: baked }

/// A size.
record Size {
  /// Its weight.
  weight: Int = 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1
}

/// A thing sold.
record Item {
  /// Its name.
  name: String
  /// Its size, whose default builds a Size from its own defaults.
  size: Size = {}
}

/// Items.
let items: table Item = {
  one { name: "x" }
}
`
	foldSweep = 60
)

// EVALUATION.md §12.2, API.md F1: E4401 in a stage-E fold of a default's record literal is located, about no value.
func TestStageEFoldBudgetLocated(t *testing.T) {
	weight, brace := strings.Index(foldSource, "1 + 1"), strings.Index(foldSource, "{}")
	defaults := [][2]int{{weight, weight + len("1 + 1 + 1 + 1 + 1 + 1 + 1 + 1")}, {brace, brace + len("{}")}}
	inDefault := func(at int) bool {
		return slices.ContainsFunc(defaults, func(d [2]int) bool { return at >= d[0] && at < d[1] })
	}
	folds := 0
	for n := 1; n <= foldSweep; n++ {
		stops, files := budgetStops(t, n)
		if len(stops) > 1 {
			t.Fatalf("budget %d: %d budget stops", n, len(stops))
		}
		for _, f := range stops {
			if files.Path(f.Span.File) != "a/a.canon" || f.Package != "a" {
				t.Errorf("budget %d: stop at %q in %q, want a/a.canon in a", n, files.Path(f.Span.File), f.Package)
			}
			if f.Path == "" && !inDefault(int(f.Span.Start)) {
				t.Errorf("budget %d: the stage-E fold stops at %d, outside the defaults %v", n, f.Span.Start, defaults)
			}
			if f.Path == "" {
				folds++
			}
		}
	}
	if folds == 0 {
		t.Error("no budget runs out in stage E's fold of a default: the sweep proves nothing")
	}
}

// budgetStops is the E4401 of a check of the fold case at budget n, and the files they lie in.
func budgetStops(t *testing.T, n int) ([]diag.Finding, diag.Files) {
	t.Helper()
	fsys := mapFS{"p/project.canon": file(fmt.Sprintf(foldProject, n)), "p/a/a.canon": file(foldSource)}
	p, err := build.Open(fsys, "/p", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Check(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var stops []diag.Finding
	for _, f := range res.List {
		if f.Code == diag.E4401.Def().Code {
			stops = append(stops, f)
		}
	}
	return stops, res.Files
}
