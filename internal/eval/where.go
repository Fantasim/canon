package eval

import (
	"context"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// Where re-runs a where predicate for stage B at no step cost, as one implicit frame (DECISIONS 148, 186, 210).
func (e *Evaluator) Where(ctx context.Context, p *types.Predicate, it value.Value) (bool, bool) {
	if p == nil || p.Expr == nil || e.info == nil {
		return false, false
	}
	file := e.index.file[p.Expr]
	r := e.newRun(ctx, charge{pkg: e.index.pkg[file]}, file)
	r.free = true
	r.fr.it = it
	if !r.nest(r.span(p.Expr)) {
		return false, false
	}
	defer r.unnest()
	holds, ok := r.truth(p.Expr)
	return holds, ok && !r.failed
}

// where runs a refinement's predicate at a storage point in its implicit frame (EVALUATION.md §12.1, DECISIONS 210).
func (r *run) where(p *types.Predicate, v value.Value) (bool, bool) {
	file := r.ev.index.file[p.Expr]
	saved := r.fr
	r.fr = (&frame{vars: map[check.Object]value.Value{}, it: v, file: file, pkg: r.ev.index.pkg[file]}).under(saved)
	defer func() { r.fr = saved }()
	if !r.nest(r.span(p.Expr)) {
		return false, false
	}
	defer r.unnest()
	return r.truth(p.Expr)
}
