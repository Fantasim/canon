package conform

import (
	"context"
	"fmt"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/value"
)

// expect evaluates each vector, in TS mode too when asked; each cut vector is one E9009 and leaves s none (CONFORMANCE.md §6.5; meta/decisions/log-2026-09-24.md).
func (f *filler) expect(ctx context.Context, s *site, inputs []input) ([]*ir.Vector, error) {
	out := make([]*ir.Vector, 0, len(inputs))
	cut := false
	for n, in := range inputs {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf(fmtCanceled, err)
		}
		v, ok, err := f.vector(ctx, s, in, n+1)
		if err != nil {
			return nil, err
		}
		if !ok {
			cut = true
			continue
		}
		out = append(out, v)
	}
	if cut {
		return nil, nil
	}
	return out, nil
}

// vector is vector number n's expectations; false when an evaluation of it was cut.
func (f *filler) vector(ctx context.Context, s *site, in input, n int) (*ir.Vector, bool, error) {
	c := Call{Fn: s.obj, Recv: in.full, Args: in.args}
	v := &ir.Vector{Recv: in.recv, Args: in.args}
	native, err := f.evaluate(ctx, c, Mode{Steps: stepCap})
	if err != nil || f.cut(s, native, n) {
		return nil, false, err
	}
	if v.Want, v.Code, err = settle(native, s); err != nil || !f.ts {
		return v, true, err
	}
	ts, err := f.evaluate(ctx, c, Mode{Steps: stepCap, TS: true})
	if err != nil || f.cut(s, ts, n) {
		return nil, false, err
	}
	v.TSWant, v.TSCode, err = settle(ts, s)
	return v, true, err
}

// evaluate is c's outcome in mode m, or the context's error when it ended during the evaluation.
func (f *filler) evaluate(ctx context.Context, c Call, m Mode) (Outcome, error) {
	o := f.ev.Evaluate(ctx, c, m)
	if err := ctx.Err(); err != nil {
		return Outcome{}, fmt.Errorf(fmtCanceled, err)
	}
	return o, nil
}

// cut reports E9009 for an evaluation a limit cut short, and whether it was.
func (f *filler) cut(s *site, o Outcome, n int) bool {
	switch o.Exceeded {
	case StepLimit:
		diag.E9009.AtSteps(s.span(), int64(n), s.label, stepCap).Report(f.bag)
		return true
	case DepthLimit:
		diag.E9009.AtDepth(s.span(), int64(n), s.label).Report(f.bag)
		return true
	case NoLimit:
	}
	return false
}

// settle is an outcome as a vector holds it: the code of its first error, else its value.
func settle(o Outcome, s *site) (value.Value, diag.Code, error) {
	switch {
	case o.Code != "":
		return nil, o.Code, nil
	case o.Value == nil:
		return nil, "", fmt.Errorf(fmtNamed, ErrNoOutcome, s.label)
	}
	return o.Value, "", nil
}

// selfFn reads a precomputed method of s's owner on its own cap: a code or a limit skips the call (meta/decisions/log-2026-09-24.md, conform N1), no outcome is internal (DECISIONS 204).
func (f *filler) selfFn(ctx context.Context, s *site) selfFn {
	return func(recv value.Value, name string) (value.Value, error) {
		obj := s.selfFns[name]
		if obj == nil {
			return nil, nil
		}
		o, err := f.evaluate(ctx, Call{Fn: obj, Recv: recv}, Mode{Steps: stepCap})
		if err != nil {
			return nil, err
		}
		switch {
		case o.Code != "" || o.Exceeded != NoLimit:
			return nil, errFailedRead
		case o.Value == nil:
			return nil, fmt.Errorf(fmtNamed, ErrNoOutcome, s.label+ownerSep+name)
		}
		return o.Value, nil
	}
}
