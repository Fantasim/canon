package eval

import (
	"context"
	"errors"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
)

// folder folds constants with the one evaluator (IMPLEMENTATION-PLAN §4.7, DECISIONS 150).
type folder struct {
	bags check.Bags
	opt  Options
	ev   *Evaluator
	bugs []error // the internal errors of every fold so far, in the order met
}

// NewFolder is the check.Folder a build gives check.Check: its findings go to the bag of the
// declaration that owns the folded expression.
func NewFolder(bags check.Bags, opt Options) check.Folder {
	return &folder{bags: bags, opt: opt}
}

// Fold evaluates e, a constant expression of owner that check has typed; a let, a user fn or a load in it fails the fold without a finding, which check reports (TYPES.md §15, DECISIONS 150).
func (f *folder) Fold(ctx context.Context, owner check.Object, e syntax.Expr, info *check.Info) (value.Value, bool) {
	if owner == nil || info == nil || f.bags == nil {
		return nil, false
	}
	if f.ev == nil || f.ev.info != info {
		f.ev = newEvaluator(f.bags, f.opt)
		f.ev.info, f.ev.constant = info, true
	}
	ev := f.ev
	ev.index.pkg[owner.File()] = owner.Pkg()
	r := ev.newRun(ctx, charge{pkg: owner.Pkg(), name: owner.Name()}, owner.File())
	v := r.eval(e)
	f.bugs, ev.bugs = append(f.bugs, ev.bugs...), nil
	return v, v != nil && !r.failed
}

// FoldErr is every internal error the folds of f met, nil when none or when f is not
// NewFolder's: a build reports it as it does the evaluator's Err (DECISIONS 195).
func FoldErr(f check.Folder) error {
	if x, ok := f.(*folder); ok {
		return errors.Join(x.bugs...)
	}
	return nil
}
