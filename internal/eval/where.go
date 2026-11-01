package eval

import (
	"context"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// Where re-runs a where predicate for stage B at no step cost, as one implicit frame (DECISIONS 148, 186, 210);
// path is the canonical path of it, the Path of a hard error the predicate raises (API.md F1).
func (e *Evaluator) Where(ctx context.Context, p *types.Predicate, it value.Value, path string) (bool, bool) {
	return e.predicate(ctx, p, it, path, nil)
}

// predicate is Where for the verification s, if any: its hard error under s's frame, its kept literals s's value's.
func (e *Evaluator) predicate(ctx context.Context, p *types.Predicate, it value.Value, path string, s *StageB) (bool, bool) {
	if p == nil || p.Expr == nil || e.info == nil {
		return false, false
	}
	file := e.fileOf(p.Expr)
	r := e.newRun(ctx, charge{pkg: e.index.pkg[file]}, file)
	r.free, r.stage = true, s
	if s != nil {
		r.outer = s.r.outer
	}
	r.fr.it, r.instPath = it, path
	if !r.nest(r.span(p.Expr)) {
		return false, false
	}
	defer r.unnest()
	holds, ok := r.truth(p.Expr)
	return holds, ok && !r.failed
}

// where runs a refinement's predicate at a storage point in its implicit frame, about at (EVALUATION.md §12.1, DECISIONS 210, API.md F1).
func (r *run) where(p *types.Predicate, v value.Value, at *vpath) (bool, bool) {
	file := r.ev.fileOf(p.Expr)
	r.noteCode(file)
	saved, cur := r.fr, r.cur
	r.fr = (&frame{it: v, file: file, pkg: r.ev.index.pkg[file], decl: true}).under(saved)
	if at != nil {
		r.cur = at
	}
	defer func() { r.fr, r.cur = saved, cur }()
	if !r.nest(r.span(p.Expr)) {
		return false, false
	}
	defer r.unnest()
	return r.truth(p.Expr)
}
