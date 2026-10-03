package edit_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// pinnedSource is the Source literal of the none a committed input writes.
const pinnedSource = "none"

// pinnedEntry is the AddEntry 881024e08ddc6c7d draws: its key and the entry's Source.
const (
	pinnedKey   = "n"
	pinnedEntry = `{ code: 0, display: "Monster Kill", param: monster }`
)

// pinned are the committed inputs of testdata/fuzz/FuzzMinimalWriteAll, each as the one
// operation it draws: its kind, path and literal, whatever their index among the project's values.
var pinned = []struct {
	name string
	op   edit.Operation
}{
	{"e9750312ecd0fcd8, a Set of a Bool beside a JSON comma", edit.Operation{Kind: edit.OpSet, Path: "features.legacycpp:props[#0].permanent", Value: edit.Bool(true)}},
	{"69982768ed81d41a, a Set of none on a multi-line item", edit.Operation{Kind: edit.OpSet, Path: "resource.adventurequest:adventureQuests.styles[#0].rewards", Value: edit.Source(pinnedSource)}},
	{"7a663892a1cbc122, a Set of none on a multi-line item", edit.Operation{Kind: edit.OpSet, Path: "resource.adventurequest:adventureQuests.hourlyTargets[#2].ratesBySpecific", Value: edit.None{}}},
	{"881024e08ddc6c7d, an AddEntry to a table whose last line is a comment", edit.Operation{Kind: edit.OpAddEntry, Path: "resource.vocab:eventTypes", Key: edit.Key(pinnedKey), Value: edit.Source(pinnedEntry)}},
}

// API.md M3, M6, E7: each committed input of FuzzMinimalWriteAll is pinned by its operation and
// path, not by a candidate index (they shift with every example), applied and written minimally.
// The corpus files stay as generic seeds drawing unrelated operations; these pins are the regressions.
func TestFuzzAllRegressionsPinned(t *testing.T) {
	dir, roots := exampleRoots(t)
	fz := openFuzzProject(t, dir, roots)
	for _, p := range pinned {
		t.Run(p.name, func(t *testing.T) {
			if _, ok := fz.byPath[p.op.Path]; !ok {
				t.Fatalf("no value at %s in the examples", p.op.Path)
			}
			ops := []edit.Operation{p.op}
			if !applyChecked(t, fz.env, fz.a, ops, oneValueSet(fz.a, ops)) {
				t.Errorf("%+v: refused, as the input was written", ops)
			}
		})
	}
}
