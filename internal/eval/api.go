package eval

import (
	"context"
	"strings"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// Call runs one precomputation of stage E on fn, recv and args (EVALUATION.md §2.3).
func (e *Evaluator) Call(ctx context.Context, fn check.Object, recv value.Value, args []value.Value) (value.Value, bool) {
	if fn == nil || e.exhausted {
		return nil, false
	}
	d, ok := fn.Decl().(*syntax.FnDecl)
	if !ok || e.broken(fn) {
		return nil, false
	}
	r := e.newRun(ctx, charge{pkg: fn.Pkg(), name: fn.Name()}, fn.File())
	texts := make([]string, len(args))
	for i, a := range args {
		texts[i] = a.CanonText()
	}
	name := fnName(fn, recv) + callOpen + strings.Join(texts, argSep) + callClose
	v := r.invokeFn(fnCall{obj: fn, self: recv, args: padded(args, len(d.Params)), site: fn.File().Span(d.Name), name: name})
	return v, v != nil && !r.failed
}

// padded is a copy of args with a nil for each parameter left to its default.
func padded(args []value.Value, params int) []value.Value {
	return append(append([]value.Value(nil), args...), make([]value.Value, max(params-len(args), 0))...)
}

// ReportUnbound is E3505 for a ref stage B found unbound (EVALUATION.md §3.4, DECISIONS 186).
func (e *Evaluator) ReportUnbound(root Root, ref *value.Ref, path string) {
	if e.vec == nil && e.bags[root.Pkg] == nil {
		return
	}
	if b := e.unbound(root, ref, path); b != nil {
		e.report(root.Pkg, b)
		e.MarkInvalid(ref)
	}
}

// ReportUnboundInto is ReportUnbound for an expect subject, into its capture (EVALUATION.md §10.2).
func (e *Evaluator) ReportUnboundInto(root Root, ref *value.Ref, path string, capture *diag.Bag) {
	if b := e.unbound(root, ref, path); b != nil && capture != nil {
		b.Report(capture)
		e.MarkInvalid(ref)
	}
}

// unbound is the E3505 finding of an unbound ref, located at its origin; nil for a non-ref.
func (e *Evaluator) unbound(root Root, ref *value.Ref, path string) *diag.Builder {
	rt, ok := ref.T.Base().(*types.RefType)
	if !ok {
		return nil
	}
	b := diag.E3505.At(located(ref, e.declSpan(root)), elemName(rt.Target), ownerName(rt.Target)).Path(path)
	if p := origin(ref.Prov()); p != nil {
		b.Pointer(p.Pointer).Layer(p.Layer).Stack(p.Stack).MoreFrames(p.MoreFrames)
	}
	return b
}

// declSpan is the declaration of a top-level value, where a finding without a value location goes.
func (e *Evaluator) declSpan(root Root) source.Span {
	if st := e.rootState(root); st != nil && st.obj.File() != nil {
		return st.obj.File().Span(st.obj.Decl())
	}
	return source.Span{}
}
