package check_test

import (
	"context"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
)

// countingFolder counts the folds it is asked for, answering as the evaluator's folder does.
type countingFolder struct {
	inner check.Folder
	calls int
}

func (c *countingFolder) Fold(ctx context.Context, owner check.Object, e syntax.Expr, info *check.Info) (value.Value, bool) {
	c.calls++
	return c.inner.Fold(ctx, owner, e, info)
}

// IMPLEMENTATION-PLAN §4.7, DECISIONS 104: a swapped file that folds ends the lineage before the Folder is asked or a finding reported.
func TestRecheckRefusesASwappedFileThatFolds(t *testing.T) {
	w := newWorld(t, priced)
	_, s, _ := w.session()
	check.UncompleteRecords(s)
	nf := w.edit("p/a.canon", "price: LIMIT - 1", "price: LIMIT - 2")
	bags := check.Bags{}
	w.parseInto(bags, nf)
	before := 0
	for _, b := range bags { //canon:unordered a count
		before += len(b.Findings())
	}
	fold := &countingFolder{inner: eval.NewFolder(bags, eval.Options{})}
	if _, _, ok := s.Recheck(t.Context(), []*syntax.File{nf}, bags, fold); ok {
		t.Fatal("Recheck held although the swapped file folded")
	}
	if fold.calls != 0 {
		t.Errorf("the Folder was asked %d times by an aborted Recheck, want 0 (nothing charged)", fold.calls)
	}
	after := 0
	for _, b := range bags { //canon:unordered a count
		after += len(b.Findings())
	}
	if after != before {
		t.Errorf("an aborted Recheck reported %d findings into the caller's bags", after-before)
	}
	if _, _, _, ok := w.recheck(s, w.edit("p/a.canon", "price: LIMIT - 1", "price: 7")); ok {
		t.Fatal("the lineage went on after the guard ended it")
	}
}
