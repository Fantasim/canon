package eval_test

import (
	"context"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
)

// cancelUser is package a: one bound that takes more steps than the evaluator runs between two looks at its context.
const cancelUser = "/// A.\npackage a\n\n/// A thing sold.\nrecord Item {\n  /// Its price.\n" +
	"  price: Int(0..=[i for i in 0..20000].len())\n}\n"

// cancelAfter is a context whose Err turns non-nil from its first call on.
type cancelAfter struct{ context.Context }

func (cancelAfter) Err() error { return context.Canceled }

// cancellingFolder folds each expression on a context that cancels part-way through the fold.
type cancellingFolder struct{ inner check.Folder }

func (f cancellingFolder) Fold(ctx context.Context, owner check.Object, e syntax.Expr, info *check.Info) (value.Value, bool) {
	return f.inner.Fold(cancelAfter{ctx}, owner, e, info)
}

// notConstantMessage is the rendered message of the notConstant variant at a refinement bound.
func notConstantMessage() string {
	bag := diag.NewBag(&source.FileSet{}, "")
	diag.E3015.AtNotConstant(source.Span{}, diag.KindRefinementBound).Report(bag)
	return bag.Findings()[0].Message
}

// truncUser is E3015's two-bound source of budget 5: the first fold spends the last step in a.
const truncUser = "/// A.\npackage a\n\n/// The base price.\nconst BASE = 100\n\n/// The highest price.\nconst LIMIT = BASE + 1 + 1 + 1\n\n" +
	"/// A thing sold.\nrecord Item {\n  /// Its price.\n  price: Int(0..=LIMIT)\n}\n\n/// Another thing.\nrecord Other {\n  /// Its count.\n  count: Int(0..=9)\n}\n"

// truncDup declares BAD twice, so an E2106 sorts ahead of the E4401 in a's bag.
const truncDup = "/// A.\npackage a\n\n/// One.\nconst BAD = 1\n\n/// Two.\nconst BAD = 2\n"

const (
	truncBudget = 5
	truncSmall  = 1    // findings the bag keeps: E4401 is cut from its view
	truncWide   = 1000 // findings it keeps: the full view
	truncLater  = 2    // the bounds of Other, folded after the spender, whose own E4401 stands in a's bag
)

// truncErrors is the errors a check of truncUser counts in a's bag, kept at most keep findings,
// and the budget findings of its full view.
func truncErrors(t *testing.T, keep int) (errors, budget int) {
	t.Helper()
	p := parseFiles(t, []string{"a/0.canon", "a/a.canon"}, [][]byte{[]byte(truncDup), []byte(truncUser)})
	bags := check.Bags{"a": diag.NewBag(p.fs, "a")}
	bags["a"].Truncate(keep)
	check.Check(context.Background(), exampleProject(), p.files, bags, eval.NewFolder(bags, eval.Options{Budget: truncBudget}))
	for _, f := range bags["a"].Findings() {
		if f.Code == diag.E3015.Def().Code && f.Message == budgetMessage() {
			budget++
		}
	}
	return bags["a"].Summary().Errors, budget
}

// API.md F7, DECISIONS 263: the findings a build reports do not depend on how many its bag keeps.
func TestBudgetFindingIgnoresTruncation(t *testing.T) {
	small, _ := truncErrors(t, truncSmall)
	wide, budget := truncErrors(t, truncWide)
	if small != wide {
		t.Errorf("%d errors kept at %d findings, %d at %d", small, truncSmall, wide, truncWide)
	}
	if budget != truncLater {
		t.Errorf("%d budget findings, want the %d folds after the spender", budget, truncLater)
	}
}

// DECISIONS 263: a fold cancelled with the counter unspent is notConstant, never budget.
func TestFoldCancelledIsNotConstant(t *testing.T) {
	p := parseFiles(t, []string{"a/a.canon"}, [][]byte{[]byte(cancelUser)})
	bags := check.Bags{}
	check.Check(context.Background(), exampleProject(), p.files, bags, cancellingFolder{eval.NewFolder(bags, eval.Options{})})
	var got []string
	for _, f := range bags["a"].Findings() {
		if f.Code == diag.E3015.Def().Code {
			got = append(got, f.Message)
		}
	}
	if len(got) == 0 || got[0] != notConstantMessage() {
		t.Errorf("findings %q, want the notConstant message %q", got, notConstantMessage())
	}
	for _, m := range got {
		if m == budgetMessage() {
			t.Errorf("a cancelled fold got the budget variant: %q", m)
		}
	}
}
