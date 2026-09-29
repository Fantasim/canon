package eval_test

import (
	"context"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/value"
)

// sumSource folds two refinement bounds during the check and evaluates a let that loops.
const sumSource = `/// A.
package a

/// One.
const ONE = 1

/// The highest price.
const LIMIT = ONE + ONE + ONE + ONE

/// A thing sold.
record Item {
  /// Its price.
  price: Int(0..=LIMIT)
  /// Its stock.
  stock: Int(0..=LIMIT + ONE)
}

local fn loop(n: Int) -> Int {
  var i = 0
  while i < n { i += 1 }
  return i
}

/// One item.
let one: Item = { price: loop(3), stock: 2 }
`

// spending is one checked and evaluated run of sumSource: the steps its folds spent during the
// check, the steps of the whole run, and how many E4401 it reported.
type spending struct {
	folds, total int64
	e4401        int
}

// DECISIONS 104, EVALUATION.md §12.2: the folds and stages A-D spend one counter, needing their sum.
func TestOneCounterSumsFoldsAndEvaluation(t *testing.T) {
	p := parseFiles(t, []string{"a/a.canon"}, [][]byte{[]byte(sumSource)})
	whole := spend(t, p, 0)
	evaluated := whole.total - whole.folds
	if whole.folds <= 1 || evaluated <= 1 || whole.e4401 != 0 {
		t.Fatalf("want folds and evaluation spending steps each, the budget kept: %+v", whole)
	}
	t.Logf("the folds spend %d steps, the evaluation %d", whole.folds, evaluated)
	need := whole.total + 1 // spending the last step is E4401
	for n := int64(1); n <= need; n++ {
		got := spend(t, p, n)
		want := 1
		if n == need {
			want = 0
		}
		if got.e4401 != want {
			t.Errorf("budget %d of %d needed: %+v, want %d exhausted", n, need, got, want)
		}
	}
	if parts := max(whole.folds, evaluated) + 1; spend(t, p, parts).e4401 != 1 {
		t.Errorf("budget %d fits each part alone: want the one counter exhausted", parts)
	}
}

// spend checks and evaluates p under budget n, the folder and the evaluator on one counter, as a build.
func spend(t *testing.T, p *program, n int64) spending {
	t.Helper()
	ctx := context.Background()
	opt := eval.Options{Budget: n}
	b := &build{prog: p, bags: check.Bags{}, values: map[eval.Root]value.Value{}}
	fold := eval.NewFolder(b.bags, opt)
	b.checked = check.Check(ctx, exampleProject(), p.files, b.bags, fold)
	out := spending{folds: eval.FoldSteps(fold)}
	b.evaluate(ctx, &host{}, opt, nil, func(ev *eval.Evaluator) { ev.UseFolder(fold) })
	out.total = eval.FoldSteps(fold)
	for _, pkg := range b.checked.Packages {
		for _, f := range b.bags[pkg.Path].Findings() {
			if f.Code == diag.E4401.Def().Code {
				out.e4401++
			}
		}
	}
	return out
}

// DECISIONS 104: joining a folder after evaluating is a no-op that keeps the steps charged; another Folder is refused.
func TestUseFolderLate(t *testing.T) {
	ctx := context.Background()
	p := parseFiles(t, []string{"a/a.canon"}, [][]byte{[]byte(sumSource)})
	bags := check.Bags{}
	fold := eval.NewFolder(bags, eval.Options{})
	prog := check.Check(ctx, exampleProject(), p.files, bags, fold)
	ev := eval.New(prog, served{}, bags, eval.Options{})
	if ev.UseFolder(otherFolder{}) {
		t.Error("a folder that is not NewFolder's was joined")
	}
	ev.Force(ctx, eval.Root{Pkg: "a", Name: "one"})
	spent := ev.StepsSpent()
	if ev.UseFolder(fold) || ev.StepsSpent() != spent || spent == 0 {
		t.Errorf("a late UseFolder took effect: %d steps, then %d", spent, ev.StepsSpent())
	}
}
