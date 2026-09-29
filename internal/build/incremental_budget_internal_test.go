package build

import (
	"fmt"
	"path"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
)

const (
	budgetSamples = 24
	budgetEdits   = 3
	budgetCeiling = 1 << 16
)

// budgeted is the entries case with project.canon's budget set to n (EVALUATION.md §12.2).
func budgeted(t *testing.T, n int) *analyzer {
	t.Helper()
	z := archiveAnalyzer(t, entriesCase)
	z.fs.set(path.Join(archiveRoot, "project.canon"), fmt.Appendf(nil, "project acme {\n  canon: \"0.1\"\n  budget: %d\n}\n", n))
	return z
}

// exhausted reports a cold analysis at budget n that runs out of steps (E4401).
func exhausted(t *testing.T, n int) bool {
	t.Helper()
	_, cold := budgeted(t, n).pair(t)
	return slices.ContainsFunc(cold.Result().List, func(f diag.Finding) bool { return f.Code == diag.E4401.Def().Code })
}

// EVALUATION.md §12.2, IMPLEMENTATION-PLAN §7.6 NFR-02: at every budget, replayed entries stop as cold ones.
func TestIncrementalBudgetCuts(t *testing.T) {
	low, high := 1, budgetCeiling
	for low < high { // the least budget the cold analysis does not exhaust
		mid := (low + high) / 2
		if exhausted(t, mid) {
			low = mid + 1
		} else {
			high = mid
		}
	}
	if low == budgetCeiling {
		t.Fatalf("still exhausted at %d steps", budgetCeiling)
	}
	t.Logf("a cold analysis needs %d steps", low)
	for i := range budgetSamples {
		n := 1 + i*low/(budgetSamples-1)
		z := budgeted(t, n)
		entries := entryFiles(t, z)
		e := newEditor(t, z, entries, entries)
		e.run(t, budgetEdits)
	}
}
