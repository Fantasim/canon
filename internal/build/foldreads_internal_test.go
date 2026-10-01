package build

import (
	"fmt"
	"strconv"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
)

const (
	foldReadCase = "testdata/incremental/foldread.txtar"
	foldReadPkg  = "a" // P's bound folds b.N; nothing a evaluates reads b
	foldReadLib  = "b"
	foldReadRuns = 2 // recorded, then replayed
)

// foldReadRoots are the constants only P's bound reads: b.N, and b.L through it.
var foldReadRoots = []eval.Root{{Pkg: foldReadLib, Name: "N"}, {Pkg: foldReadLib, Name: "L"}}

// DECISIONS 264, EVALUATION.md §2.1, §5: b.L's orphan ref is E3505 whatever the selection, warm as cold.
func TestFoldReadsVerified(t *testing.T) {
	for _, sel := range [][]string{nil, {foldReadPkg}, {foldReadLib}} {
		t.Run(fmt.Sprint(sel), func(t *testing.T) {
			z := archiveAnalyzer(t, foldReadCase)
			for i := range foldReadRuns {
				warm, cold := z.pairSel(t, sel)
				same(t, strconv.Itoa(i), warm, cold)
				if got := counts(cold, diag.E3505.Def().Code); got != 1 {
					t.Errorf("run %d: %d orphan refs, want 1: %v", i, got, cold.Result().List)
				}
			}
		})
	}
}

// DECISIONS 244, 264, EVALUATION.md §12.2: a fold's reads are charged again in stage A; E4401 as cold.
func TestFoldReadsCharged(t *testing.T) {
	// Under a alone, b.N and b.L cost their fold and stage A's force: twice what b alone charges,
	// which folds neither. At every budget up to the need, one E4401 lands where cold puts it.
	top := foldBudgetTop * foldBudgetTop
	_, once := withBudget(t, foldReadCase, top).pairSel(t, []string{foldReadLib})
	_, twice := withBudget(t, foldReadCase, top).pairSel(t, []string{foldReadPkg})
	for _, root := range foldReadRoots {
		if n := spentOn(once, root); n == 0 || spentOn(twice, root) != 2*n {
			t.Errorf("%v: %d steps under a, %d under b: want twice b's", root, spentOn(twice, root), n)
		}
	}
	need := int(charged(twice)) + 1 // spending the last step is E4401
	if counts(twice, diag.E4401.Def().Code) != 0 {
		t.Fatalf("budget %d runs out", top)
	}
	for n := 1; n <= need; n++ {
		warm, cold := withBudget(t, foldReadCase, n).pairSel(t, []string{foldReadPkg})
		same(t, strconv.Itoa(n), warm, cold)
		want := 1
		if n == need {
			want = 0
		}
		if got := counts(cold, diag.E4401.Def().Code); got != want {
			t.Errorf("budget %d of %d needed: %d exhausted, want %d", n, need, got, want)
		}
	}
}

// DECISIONS 264, IMPLEMENTATION-PLAN §7.6: under a alone, entry edits recheck along one lineage as cold.
func TestFoldReadsLineage(t *testing.T) {
	_, whole := withBudget(t, foldReadCase, foldBudgetTop*foldBudgetTop).pairSel(t, []string{foldReadPkg})
	need := int(charged(whole)) + 1
	for n := 1; n <= need; n++ {
		t.Run(strconv.Itoa(n), func(t *testing.T) {
			z := withBudget(t, foldReadCase, n)
			z.sel = []string{foldReadPkg}
			entries := entryFiles(t, z)
			e := newEditor(t, z, entries, entries)
			e.run(t, foldEdits)
			if e.lineage == 0 {
				t.Error("no edit was re-checked along the lineage")
			}
		})
	}
}

// spentOn is the steps a's one counter charged root.
func spentOn(a *Analysis, root eval.Root) int64 {
	for _, c := range a.r.ev.Charged() {
		if c.Pkg == root.Pkg && c.Name == root.Name {
			return c.Steps
		}
	}
	return 0
}
