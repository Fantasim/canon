package edit_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// pinnedSource is the Source literal of the none a committed input writes.
const pinnedSource = "none"

// pinned are the committed inputs of testdata/fuzz/FuzzMinimalWriteAll, each as the one Set it
// draws: a path and a literal, whatever their index among the project's values.
var pinned = []struct {
	name string
	path string
	lit  edit.Lit
}{
	{"e9750312ecd0fcd8, a Set of a Bool beside a JSON comma", "features.legacycpp:props[#0].permanent", edit.Bool(true)},
	{"69982768ed81d41a, a Set of none on a multi-line item", "resource.adventurequest:adventureQuests.styles[#0].rewards", edit.Source(pinnedSource)},
	{"7a663892a1cbc122, a Set of none on a multi-line item", "resource.adventurequest:adventureQuests.hourlyTargets[#2].ratesBySpecific", edit.None{}},
}

// API.md M6, E7: each committed input of FuzzMinimalWriteAll is pinned by its operation and path,
// not by a candidate index (they shift with every example), applied and written minimally. The
// corpus files stay as generic seeds drawing unrelated operations; these pins are the regressions.
func TestFuzzAllRegressionsPinned(t *testing.T) {
	dir, roots := exampleRoots(t)
	fz := openFuzzProject(t, dir, roots)
	for _, p := range pinned {
		t.Run(p.name, func(t *testing.T) {
			if _, ok := fz.byPath[p.path]; !ok {
				t.Fatalf("no value at %s in the examples", p.path)
			}
			ops := []edit.Operation{{Kind: edit.OpSet, Path: p.path, Value: p.lit}}
			if !applyChecked(t, fz.env, fz.a, ops, oneValueSet(fz.a, ops)) {
				t.Errorf("%+v: refused, as the input was written", ops)
			}
		})
	}
}
