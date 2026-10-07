package rules

import (
	"context"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
)

// underRunner is what an Evaluator running a precomputed result's checks under its frame also does.
type underRunner interface {
	RunUnder(ctx context.Context, c *syntax.CheckDecl, self value.Value, f diag.Frame) Run
}

// Result runs the instance checks of v, a precomputed result of pkg, under frame (DECISIONS 324).
func (r *Runner) Result(ctx context.Context, pkg string, v value.Value, frame diag.Frame) error {
	bag, err := r.bagOf(pkg)
	if err != nil {
		return err
	}
	t := &traversal{Runner: r, ctx: ctx, bag: bag, rootOf: eval.Root{Pkg: pkg}, under: &frame}
	defer t.chargeTo()()
	t.visit(v, nil)
	return nil
}

// runUnder runs c on rec, an instance of a precomputed result, under its frame when the evaluator can.
func (t *traversal) runUnder(c *syntax.CheckDecl, rec *value.Record) Run {
	if u, ok := t.ev.(underRunner); ok {
		return u.RunUnder(t.ctx, c, rec, *t.under)
	}
	return t.ev.Run(t.ctx, c, rec, "")
}
