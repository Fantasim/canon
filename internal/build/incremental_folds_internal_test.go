package build

import (
	"fmt"
	"path"
	"slices"
	"strconv"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
)

const (
	foldsCase     = "testdata/incremental/folds.txtar"
	foldBudgetTop = 48 // past what the folds case spends cold, in its folder as in its evaluator
	foldEdits     = 4
)

// EVALUATION.md §12.2, DECISIONS 104, IMPLEMENTATION-PLAN §7.6 NFR-02: warm E4401 is cold's.
func TestIncrementalFoldBudget(t *testing.T) {
	for n := 1; n <= foldBudgetTop; n++ {
		t.Run(strconv.Itoa(n), func(t *testing.T) {
			z := foldsAt(t, n)
			entries := entryFiles(t, z)
			e := newEditor(t, z, entries, entries)
			e.run(t, foldEdits)
			if e.lineage == 0 {
				t.Error("no edit was re-checked along the lineage")
			}
		})
	}
	_, cold := foldsAt(t, foldBudgetTop).pair(t)
	if slices.ContainsFunc(cold.Result().List, func(f diag.Finding) bool { return f.Code == diag.E4401.Def().Code }) {
		t.Errorf("budget %d runs out", foldBudgetTop)
	}
}

// foldsAt is the folds case with project.canon's budget set to n (EVALUATION.md §12.2).
func foldsAt(t *testing.T, n int) *analyzer {
	t.Helper()
	z := archiveAnalyzer(t, foldsCase)
	z.fs.set(path.Join(archiveRoot, "project.canon"), fmt.Appendf(nil, "project acme {\n  canon: \"0.1\"\n  budget: %d\n}\n", n))
	return z
}
