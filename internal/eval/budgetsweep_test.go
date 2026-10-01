package eval_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/source"
)

// sweepUser is package a: four bounds fold in source order, the second one reading b's LIMIT.
const sweepUser = "/// A.\npackage a\n\nimport b\n\n/// A thing sold.\nrecord Item {\n  /// Its price.\n" +
	"  price: Int(0..=b.LIMIT)\n  /// Its stock.\n  stock: Int(0..=9)\n}\n"

// sweepLib is package b: LIMIT costs the heaviest steps, and only a bound of a reads it.
const sweepLib = "/// B.\npackage b\n\n/// The base price.\nconst BASE = 100\n\n/// The highest price.\nconst LIMIT = BASE + 1 + 1 + 1\n"

const (
	sweepUserPkg = "a"
	sweepLibPkg  = "b"
	sweepMax     = 18 // a budget past the steps the whole program spends
	sweepBounds  = 4  // price's two bounds, stock's two
	limitBound   = 1  // the bound reading b.LIMIT, the only fold that evaluates a constant of b
)

// sweepBoundOffsets is where each bound of sweepUser starts, in fold order.
func sweepBoundOffsets() []source.Pos {
	var out []source.Pos
	for _, at := range []string{"0..=b.LIMIT", "b.LIMIT", "0..=9", "9)"} {
		out = append(out, source.Pos(strings.Index(sweepUser, at)))
	}
	return out
}

// budgetMessage is the rendered message of the budget variant at a refinement bound.
func budgetMessage() string {
	bag := diag.NewBag(&source.FileSet{}, "")
	diag.E3015.AtBudget(source.Span{}, diag.KindRefinementBound).Report(bag)
	return bag.Findings()[0].Message
}

// sweepRun builds the two packages on a budget of n steps.
func sweepRun(t *testing.T, n int64) *build {
	t.Helper()
	p := parseFiles(t, []string{"a/a.canon", "b/b.canon"}, [][]byte{[]byte(sweepUser), []byte(sweepLib)})
	return runBuild(t, p, eval.Options{Budget: n})
}

// budgetFindings are the starts of the budget findings of a build and the starts of the stops by package.
func budgetFindings(t *testing.T, b *build, want string) (starts []source.Pos, stops map[string][]source.Pos) {
	t.Helper()
	stops = map[string][]source.Pos{}
	for pkg, bag := range b.bags {
		for _, f := range bag.Findings() {
			if f.Code == diag.E4401.Def().Code {
				stops[pkg] = append(stops[pkg], f.Span.Start)
			}
			if f.Code != diag.E3015.Def().Code {
				continue
			}
			starts = append(starts, f.Span.Start)
			if f.Message != want {
				t.Errorf("package %s: %q, want the budget variant %q", pkg, f.Message, want)
			}
		}
	}
	slices.Sort(starts)
	return starts, stops
}

// DECISIONS 263: over every budget the budget variant stands on a suffix of the folds, never otherwise.
func TestBudgetSweep(t *testing.T) {
	budgetMsg := budgetMessage()
	bounds := sweepBoundOffsets()
	stopped := 0
	for n := int64(1); n <= sweepMax; n++ {
		starts, stops := budgetFindings(t, sweepRun(t, n), budgetMsg)
		total := len(stops[sweepUserPkg]) + len(stops[sweepLibPkg])
		if total > 1 {
			t.Errorf("budget %d: %d stops, want at most once (DECISIONS 244)", n, total)
		}
		if total == 0 {
			if len(starts) != 0 {
				t.Errorf("budget %d: findings %v with the counter unspent", n, starts)
			}
			continue
		}
		first := len(bounds) - len(starts)
		if !slices.Equal(starts, bounds[first:]) {
			t.Errorf("budget %d: findings at %v, want a suffix of the bounds %v", n, starts, bounds)
		}
		checkSpender(t, n, first, stops)
		stopped += len(starts)
	}
	if stopped < sweepBounds {
		t.Errorf("the sweep stopped %d folds, want at least %d: it proves nothing", stopped, sweepBounds)
	}
}

// checkSpender holds the fold that spent the last step to the owner's bag (DECISIONS 263): a stop
// in b is the bound reading b.LIMIT, or a later stage's; one in a is at its bound, the suffix right after.
func checkSpender(t *testing.T, n int64, first int, stops map[string][]source.Pos) {
	t.Helper()
	if len(stops[sweepLibPkg]) == 1 && first != limitBound && first != sweepBounds {
		t.Errorf("budget %d: the stop in b, the suffix starts at bound %d, want %d or none", n, first, limitBound)
	}
	for _, at := range stops[sweepUserPkg] {
		if want := slices.Index(sweepBoundOffsets(), at) + 1; want == 0 || first != want {
			t.Errorf("budget %d: the stop in a at %d, the suffix starts at bound %d, want the one after its bound", n, at, first)
		}
	}
}
