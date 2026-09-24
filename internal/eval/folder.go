package eval

import (
	"context"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
)

// folder folds constants with the one evaluator (IMPLEMENTATION-PLAN §4.7, DECISIONS 150).
type folder struct {
	bags check.Bags
	opt  Options
	ev   *Evaluator
}

// NewFolder is the check.Folder a build gives check.Check: its findings go to the bag of the
// declaration that owns the folded expression.
func NewFolder(bags check.Bags, opt Options) check.Folder {
	return &folder{bags: bags, opt: opt}
}

// Fold evaluates e, a constant expression of owner that check has typed (TYPES.md §15).
func (f *folder) Fold(ctx context.Context, owner check.Object, e syntax.Expr, info *check.Info) (value.Value, bool) {
	if owner == nil || info == nil || f.bags == nil {
		return nil, false
	}
	if f.ev == nil || f.ev.info != info {
		f.ev = newEvaluator(f.bags, f.opt)
		f.ev.info = info
	}
	ev := f.ev
	ev.index.pkg[owner.File()] = owner.Pkg()
	r := ev.newRun(ctx, charge{pkg: owner.Pkg(), name: owner.Name()}, owner.File())
	v := r.eval(e)
	return v, v != nil && !r.failed
}
