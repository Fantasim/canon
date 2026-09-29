package eval

import (
	"context"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
)

// force evaluates a const or let once, read by reader at node at (EVALUATION.md §3.1).
func (e *Evaluator) force(ctx context.Context, st *rootState, reader *run, at syntax.Node) (value.Value, bool) {
	switch st.status {
	case done:
		return st.v, true
	case poisoned:
		return nil, false
	case forcing:
		e.cycle(st, reader, at)
		return nil, false
	default:
	}
	if e.broken(st.obj) {
		e.constCycle(st.obj)
		st.status = poisoned
		return nil, false
	}
	if e.exhausted || ctx.Err() != nil {
		return nil, false
	}
	st.status = forcing
	e.stack = append(e.stack, st)
	v, ok := e.evalRoot(ctx, st)
	e.stack = e.stack[:len(e.stack)-1]
	clear(e.clean) // its temporaries are not kept alive past the root (DECISIONS 199)
	switch {
	case ok && st.status == forcing:
		st.status, st.v = done, v
		e.completed = append(e.completed, st)
		if e.verifying {
			e.queue = append(e.queue, st)
		}
	case e.late.free && ctx.Err() != nil: // phase 8: a cancelled call leaves it to a later one
		st.status = idle
		return nil, false
	default:
		st.status = poisoned
	}
	e.flush(e.flushContext(ctx))
	return st.v, st.status == done
}

// flushContext is ctx, which phase 8 does not let cut a verification short: a value verified
// in part would differ from the one a later call reads (VIEWMODEL.md J5).
func (e *Evaluator) flushContext(ctx context.Context) context.Context {
	if e.late.free {
		return context.WithoutCancel(ctx)
	}
	return ctx
}

// broken reports a declaration with a static error: it is never evaluated (TYPES.md §1).
func (e *Evaluator) broken(obj check.Object) bool {
	return e.info != nil && e.info.Broken[obj]
}

// evalRoot evaluates a const or a let, entries and conversion included (EVALUATION.md §9.3).
func (e *Evaluator) evalRoot(ctx context.Context, st *rootState) (value.Value, bool) {
	if _, known := e.index.pkg[st.obj.File()]; !known {
		e.index.pkg[st.obj.File()] = st.obj.Pkg()
	}
	r := e.newRun(ctx, charge{pkg: st.root.Pkg, name: st.root.Name}, st.obj.File())
	r.root, r.free = st, e.late.free
	loading := e.loading // a root forced while a load decodes is not part of it (Savepoint)
	e.loading = nil
	defer func() { e.loading = loading }()
	var v value.Value
	switch d := st.obj.Decl().(type) {
	case *syntax.ConstDecl:
		v = r.eval(d.Value)
	case *syntax.LetDecl:
		v = r.letValue(st, d)
	}
	return v, v != nil && !r.failed
}

// cycle is E4301 at the read closing a cycle between values (EVALUATION.md §3.2).
func (e *Evaluator) cycle(st *rootState, reader *run, at syntax.Node) {
	if reader == nil || at == nil {
		return
	}
	if st.obj.Kind() == check.ObjConst {
		e.constCycle(st.obj)
		reader.stop()
		return
	}
	var names []string
	from := len(e.stack)
	for i, s := range e.stack {
		if s == st {
			from = i
		}
	}
	for _, s := range e.stack[from:] {
		names = append(names, reader.qualified(s.root.Pkg, s.root.Name))
	}
	names = append(names, reader.qualified(st.root.Pkg, st.root.Name))
	reader.fail(diag.E4301.At(reader.span(at), names))
}
