package build

import (
	"context"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/rules"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// irHost is the run's evaluator as stage E's ir.Host; stage B has begun, so a first Force verifies.
type irHost struct {
	r *run
}

func (h irHost) Value(ctx context.Context, pkg, name string) (value.Value, bool) {
	return h.r.ev.Force(ctx, eval.Root{Pkg: pkg, Name: name})
}

// Call is one precomputation, then its result's verification (EVALUATION.md §2.3, DECISIONS 324).
func (h irHost) Call(ctx context.Context, fn check.Object, recv value.Value, args []value.Value) (value.Value, bool) {
	defer h.r.ev.ForgetCuts() // a later precomputation's frame is never an earlier one's top
	v, ok := h.r.ev.Call(ctx, fn, recv, args)
	if !ok {
		return v, false
	}
	return h.r.precomputed(ctx, fn, v, eval.CallFrame(fn, recv, args))
}

// precomputed verifies v, fn's result, then checks its instances unless poisoned (EVALUATION.md §5, §8.1).
func (r *run) precomputed(ctx context.Context, fn check.Object, v value.Value, frame diag.Frame) (value.Value, bool) {
	root := eval.Root{Pkg: fn.Pkg(), Name: fn.Name()} // charged as Call charges it (§12.2)
	res, err := r.host.verifier.CheckAs(ctx, root, v, resultType(fn, v), frame)
	if err != nil {
		r.host.errs = append(r.host.errs, internal(err))
		return nil, false
	}
	if res.Poisoned {
		return nil, false
	}
	if err := r.instances().Result(ctx, fn.Pkg(), res.Value, frame); err != nil {
		r.host.errs = append(r.host.errs, internal(err))
		return nil, false
	}
	return res.Value, res.Valid
}

// instances is stage C's runner, which checks an instance a top-level value holds once (EVALUATION.md §8.1).
func (r *run) instances() *rules.Runner {
	if r.runner == nil {
		r.runner = rules.NewShared(r.rix, checks{Evaluator: r.ev, failed: &r.failed}, r.bags)
	}
	return r.runner
}

// resultType is fn's declared return type, a `past` slot included (TYPES.md §8.4); v's own type when fn has none.
func resultType(fn check.Object, v value.Value) types.Type {
	if t := fn.Type(); t != nil {
		if f, ok := t.Base().(*types.FuncType); ok && f.Result != nil {
			return f.Result
		}
	}
	return v.Type()
}
