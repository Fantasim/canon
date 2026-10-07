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

// sweepHeavyLib is sweepLib with a LIMIT past every budget of the sweep: b always stops in its fold.
const sweepHeavyLib = "/// B.\npackage b\n\n/// The base price.\nconst BASE = 100\n\n/// The highest price.\n" +
	"const LIMIT = BASE + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1 + 1\n"

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

// sweepRun builds package a and lib, as b, on a budget of n steps per package.
func sweepRun(t *testing.T, lib string, n int64) *build {
	t.Helper()
	p := parseFiles(t, []string{"a/a.canon", "b/b.canon"}, [][]byte{[]byte(sweepUser), []byte(lib)})
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

// DECISIONS 263, 328: each package stops once at most, a's own bounds fail after a's own stop, whatever b's cost.
func TestBudgetSweep(t *testing.T) {
	budgetMsg := budgetMessage()
	stopped, starved := 0, 0
	for n := int64(1); n <= sweepMax; n++ {
		starts, stops := budgetFindings(t, sweepRun(t, sweepLib, n), budgetMsg)
		heavy, heavyStops := budgetFindings(t, sweepRun(t, sweepHeavyLib, n), budgetMsg)
		for pkg, at := range stops {
			if len(at) > 1 || len(heavyStops[pkg]) > 1 {
				t.Errorf("budget %d: package %s stops %d and %d times, want at most once (DECISIONS 328)", n, pkg, len(at), len(heavyStops[pkg]))
			}
		}
		if !slices.Equal(stops[sweepUserPkg], heavyStops[sweepUserPkg]) {
			t.Errorf("budget %d: a stops at %v, at %v with a heavier b: a's budget depends on b's", n, stops[sweepUserPkg], heavyStops[sweepUserPkg])
		}
		own := checkOwnSuffix(t, n, starts, stops[sweepUserPkg])
		if heavyOwn := checkOwnSuffix(t, n, heavy, heavyStops[sweepUserPkg]); !slices.Equal(own, heavyOwn) {
			t.Errorf("budget %d: a's own bounds fail at %v, at %v with a heavier b", n, own, heavyOwn)
		}
		if starvedLimit(t, n, heavy, heavyStops) {
			starved++
		}
		stopped += len(starts)
	}
	if stopped < sweepBounds || starved == 0 {
		t.Errorf("the sweep stopped %d folds, %d by b's budget alone: it proves nothing", stopped, starved)
	}
}

// checkOwnSuffix holds the bounds other than the one reading b.LIMIT to a's own budget: each fails,
// E3015 budget, exactly when it comes after the bound whose fold reported a's E4401 in a's bag
// (DECISIONS 263); it returns their findings.
func checkOwnSuffix(t *testing.T, n int64, starts, stops []source.Pos) []source.Pos {
	t.Helper()
	bounds := sweepBoundOffsets()
	stopAt := len(bounds)
	if len(stops) == 1 {
		if stopAt = slices.Index(bounds, stops[0]); stopAt < 0 {
			stopAt = len(bounds) // a stop after the folds
		}
	}
	var own []source.Pos
	for i, at := range bounds {
		if i == limitBound {
			continue
		}
		got, want := slices.Contains(starts, at), i > stopAt
		if got != want {
			t.Errorf("budget %d: bound %d failing %t with a's stop at bound %d, want %t", n, i, got, stopAt, want)
		}
		if got {
			own = append(own, at)
		}
	}
	return own
}

// starvedLimit holds the bound reading a LIMIT past the budget to E3015 budget, the E4401 in b's
// bag alone, unless a's own budget stopped at or before it: true when b's budget alone failed it.
func starvedLimit(t *testing.T, n int64, starts []source.Pos, stops map[string][]source.Pos) bool {
	t.Helper()
	bounds := sweepBoundOffsets()
	byA := len(stops[sweepUserPkg]) == 1 && slices.Index(bounds, stops[sweepUserPkg][0]) <= limitBound
	if len(stops[sweepLibPkg]) != 1 && !byA {
		t.Errorf("budget %d: a heavy LIMIT stopped b %d times, want once", n, len(stops[sweepLibPkg]))
	}
	failed := slices.Contains(starts, bounds[limitBound])
	if want := !byA || slices.Index(bounds, stops[sweepUserPkg][0]) < limitBound; failed != want {
		t.Errorf("budget %d: the bound reading a heavy LIMIT failing %t, want %t", n, failed, want)
	}
	return failed && !byA
}
