package build

import (
	"context"
	"fmt"
	"path"
	"reflect"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
)

// causeCases are archives with a root poisoned by a division by zero, and the packages selected.
var causeCases = []struct {
	file string
	root eval.Root
	sel  []string
}{
	{"testdata/incremental/cause.txtar", eval.Root{Pkg: "a", Name: "half"}, nil},
	{"testdata/incremental/causestagee.txtar", eval.Root{Pkg: "b", Name: "x"}, []string{"a"}},
}

// DECISIONS 104, EVALUATION.md §12.2: folds and stages A-E spend one counter, one E4401, needing their sum.
func TestFoldsOneCounter(t *testing.T) {
	_, whole := foldsAt(t, foldBudgetTop).pair(t)
	need := int(charged(whole)) + 1 // spending the last step is E4401
	if need > foldBudgetTop || counts(whole, diag.E4401.Def().Code) != 0 {
		t.Fatalf("the folds case needs %d steps, past %d", need, foldBudgetTop)
	}
	t.Logf("the folds case needs %d steps", need)
	for n := 1; n <= need; n++ {
		_, cold := foldsAt(t, n).pair(t)
		want := 1
		if n == need {
			want = 0
		}
		if got := counts(cold, diag.E4401.Def().Code); got != want {
			t.Errorf("budget %d of %d needed: %d exhausted, want %d", n, need, got, want)
		}
	}
}

// DECISIONS 104, EVALUATION.md §12.2, API.md R6, IMPLEMENTATION-PLAN §7.6: the cause re-run's causes are cold's.
func TestCauseRunAfterFolds(t *testing.T) {
	for _, c := range causeCases {
		t.Run(path.Base(c.file), func(t *testing.T) {
			_, whole := withBudget(t, c.file, foldBudgetTop*foldBudgetTop).pairSel(t, c.sel)
			need := int(charged(whole)) + 1
			if counts(whole, diag.E4401.Def().Code) != 0 {
				t.Fatalf("budget %d runs out", foldBudgetTop*foldBudgetTop)
			}
			unreached := 0
			for n := 1; n <= need; n++ {
				unreached += causeAt(t, withBudget(t, c.file, n), c.sel, c.root)
			}
			if unreached == 0 || unreached == need {
				t.Errorf("%d budgets of %d leave %v unevaluated: want some, not all", unreached, need, c.root)
			}
		})
	}
}

// causeAt compares root's cause warm and cold; 1 when cold finds none, root left unevaluated.
func causeAt(t *testing.T, z *analyzer, sel []string, root eval.Root) int {
	t.Helper()
	warm, cold := z.pairSel(t, sel)
	if !warm.r.memoized {
		t.Fatal("the warm analysis replays no memo")
	}
	got, err := warm.Cause(context.Background(), root)
	want, _ := cold.Cause(context.Background(), root)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Errorf("budget %d: cause %v, %v; cold %v", cold.r.s.proj.Budget, got, err, want)
	}
	if len(want) == 0 {
		return 1
	}
	return 0
}

// withBudget is the archive's analyzer with project.canon's budget set to n (EVALUATION.md §12.2).
func withBudget(t *testing.T, file string, n int) *analyzer {
	t.Helper()
	z := archiveAnalyzer(t, file)
	z.fs.set(path.Join(archiveRoot, "project.canon"), fmt.Appendf(nil, "project acme {\n  canon: \"0.1\"\n  budget: %d\n}\n", n))
	return z
}

// charged is every step a's one counter was charged, folds included.
func charged(a *Analysis) int64 {
	var n int64
	for _, c := range a.r.ev.Charged() {
		n += c.Steps
	}
	return n
}

// counts is how many findings of a carry code.
func counts(a *Analysis, code diag.Code) int {
	n := 0
	for _, f := range a.Result().List {
		if f.Code == code {
			n++
		}
	}
	return n
}
