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
	if fn == nil || e.halted(fn.Pkg()) {
		return nil, false
	}
	d, ok := fn.Decl().(*syntax.FnDecl)
	if !ok || e.broken(fn) {
		return nil, false
	}
	r := e.newRun(ctx, charge{pkg: fn.Pkg(), name: fn.Name()}, fn.File())
	r.notesCuts = true
	f := CallFrame(fn, recv, args)
	v := r.invokeFn(fnCall{obj: fn, self: recv, args: padded(args, len(d.Params)), site: f.Span, name: f.Fn})
	return v, v != nil && !r.failed
}

// CallFrame is the frame Call runs fn under: fn with its receiver and inputs, at its name (EVALUATION.md §2.3).
func CallFrame(fn check.Object, recv value.Value, args []value.Value) diag.Frame {
	texts := make([]string, len(args))
	for i, a := range args {
		texts[i] = a.CanonText()
	}
	f := diag.Frame{Fn: fnName(fn, recv) + callOpen + strings.Join(texts, argSep) + callClose}
	if d, ok := fn.Decl().(*syntax.FnDecl); ok && fn.File() != nil {
		f.Span = fn.File().Span(d.Name)
	}
	return f
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
	if b := e.unbound(root, ref, path, nil); b != nil {
		e.report(root.Pkg, b)
		e.MarkInvalid(ref)
	}
}

// ReportUnboundInto is ReportUnbound for an expect subject, into its capture (EVALUATION.md §10.2).
func (e *Evaluator) ReportUnboundInto(root Root, ref *value.Ref, path string, capture *diag.Bag) {
	if b := e.unbound(root, ref, path, nil); b != nil && capture != nil {
		b.Report(capture)
		e.MarkInvalid(ref)
	}
}

// unbound is the E3505 finding of an unbound ref, located at its origin, under outer; nil for a non-ref.
func (e *Evaluator) unbound(root Root, ref *value.Ref, path string, outer *diag.Frame) *diag.Builder {
	rt, ok := ref.T.Base().(*types.RefType)
	if !ok {
		return nil
	}
	b := diag.E3505.At(located(ref, e.declSpan(root)), elemName(rt.Target), ownerName(rt.Target)).Path(path)
	var stack []diag.Frame
	more := 0
	if p := origin(ref.Prov()); p != nil {
		b.Pointer(p.Pointer).Layer(p.Layer)
		stack, more = p.Stack, p.MoreFrames
	}
	stack, more = Outermost(stack, more, outer, e.Hides(stack, outer))
	return b.Stack(stack).MoreFrames(more)
}

// declSpan is the declaration of a top-level value, where a finding without a value location goes.
func (e *Evaluator) declSpan(root Root) source.Span {
	if st := e.rootState(root); st != nil && st.obj.File() != nil {
		return st.obj.File().Span(st.obj.Decl())
	}
	return source.Span{}
}
