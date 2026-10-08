package eval

import (
	"maps"
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// depCtx is what a type argument names where a value sits: a record, its arguments, binders,
// the declared type of the field given a value (fnArg), and where a dereference of an argument
// is reported, never nil; with the field's scope, which a load in its value decodes in (loadscope.go).
type depCtx struct {
	rec     *value.Record
	params  map[*types.Param]value.Value
	binders map[string]value.Value
	field   types.Type
	scope   *fieldScope
	at      syntax.Node
}

// withBinder is cx with a dependent map's binder bound to one of its keys (TYPES.md §11.5).
func (cx *depCtx) withBinder(name string, key value.Value, at syntax.Node) *depCtx {
	out := &depCtx{binders: map[string]value.Value{}, at: at}
	if cx != nil {
		out.rec, out.params, out.field, out.scope = cx.rec, cx.params, cx.field, cx.scope
		maps.Copy(out.binders, cx.binders)
	}
	out.binders[name] = key
	return out
}

// forField is cx giving field f its value, written as expr.
func (cx *depCtx) forField(f *types.Field, expr syntax.Expr) *depCtx {
	out := *cx
	out.field, out.scope = f.Type, newFieldScope(f, expr)
	return &out
}

// below is cx for an element or map value below its field, which takes the field's unit and int (loadscope.go).
func (cx *depCtx) below() *depCtx {
	if cx.scope == nil {
		return cx
	}
	out := *cx
	out.scope = &fieldScope{f: cx.scope.f, expr: cx.scope.expr, under: true}
	return &out
}

// unscoped is cx in no field's scope.
func (cx *depCtx) unscoped() *depCtx {
	if cx.scope == nil {
		return cx
	}
	out := *cx
	out.scope = nil
	return &out
}

// bindParams keeps an applied record instance's arguments, which its value cannot hold (DECISIONS 147).
func (e *Evaluator) bindParams(rec *value.Record, params map[*types.Param]value.Value) {
	if len(params) > 0 {
		e.bound[rec] = params
		e.noteBind(rec)
	}
}

// noteBind records a binding made during a decoding attempt, which undoing it drops (Savepoint).
func (e *Evaluator) noteBind(rec *value.Record) {
	if ld := e.loading; ld != nil && ld.save != nil {
		ld.save.binds = append(ld.save.binds, rec)
	}
}

// boundParams is the arguments bound to rec, nil for none; a vector reads its parent's.
func (e *Evaluator) boundParams(rec *value.Record) map[*types.Param]value.Value {
	if p, ok := e.bound[rec]; ok {
		return p
	}
	if e.parent != nil {
		return e.parent.boundParams(rec)
	}
	return nil
}

// shareParams gives to, a record rebuilt from from, the arguments from is bound to.
func (e *Evaluator) shareParams(from, to value.Value) {
	src, ok := from.(*value.Record)
	dst, isRec := to.(*value.Record)
	if !ok || !isRec {
		return
	}
	if p := e.boundParams(src); p != nil && e.bound[dst] == nil {
		e.bound[dst] = p
		e.noteBind(dst)
	}
}

// applyRecord binds rec's arguments when t is an application R(args) cx can read; false: aborted (TYPES.md §11.1).
func (r *run) applyRecord(rec *value.Record, t types.Type, cx *depCtx) bool {
	if app, ok := t.Base().(*types.AppliedRecord); ok {
		r.ev.bindParams(rec, r.applyArgs(app.Rec.Params, app.Args, cx))
	}
	return !r.failed
}

// applyArgs is each argument's value in cx by parameter, refs kept; nil when cx lacks one (TYPES.md §11.1).
func (r *run) applyArgs(params []*types.Param, args []*types.Arg, cx *depCtx) map[*types.Param]value.Value {
	if len(params) != len(args) || cx == nil {
		return nil
	}
	out := make(map[*types.Param]value.Value, len(params))
	for i, a := range args {
		v := r.argValue(a, cx)
		if v == nil {
			return nil
		}
		out[params[i]] = v
	}
	return out
}

// argValue is a stable path from a parameter, an earlier field or a binder, read in cx; nil
// when cx does not hold its root.
func (r *run) argValue(a *types.Arg, cx *depCtx) value.Value {
	var v value.Value
	path := a.Path
	switch {
	case a.Source == types.ArgParam && cx.params[a.Param] != nil:
		v = cx.params[a.Param]
	case a.Source == types.ArgParam:
		v = r.fnArg(cx, a.Param)
	case a.Source == types.ArgKey:
		v = cx.binders[a.Binder]
	case len(path) > 0 && cx.rec != nil && path[0].Index < len(cx.rec.Fields):
		v, path = cx.rec.Fields[path[0].Index], path[1:]
	}
	if v == nil {
		return nil
	}
	return r.follow(v, path, cx.at)
}

// follow reads fields along path through refs, dereferenced at at; nil past a missing field or an abort.
func (r *run) follow(v value.Value, path []*types.Field, at syntax.Node) value.Value {
	for _, f := range path {
		rec, ok := r.deref(v, at).(*value.Record)
		if !ok || f.Index >= len(rec.Fields) {
			return nil
		}
		v = rec.Fields[f.Index]
	}
	return v
}

// paramValue is a parameter of self's record read at at: this instance's argument, read through a ref (TYPES.md §11.1).
func (r *run) paramValue(obj check.Object, at syntax.Node) (value.Value, bool) {
	rec, ok := r.fr.self.(*value.Record)
	decl, isParam := obj.Decl().(*syntax.Param)
	if !ok || !isParam {
		return nil, false
	}
	p := recordParam(recordOf(rec.T), decl)
	v := r.ev.boundParams(rec)[p]
	if p == nil || v == nil {
		return nil, false
	}
	if recordOf(p.Type) != nil {
		v = r.deref(v, at) // nil once the root is aborted (E3501, E3505, E4301)
	}
	return v, true
}

// ownParam reports obj a parameter of self's record; unbound in a fold, it is not constant (TYPES.md §11.1, §15).
func (r *run) ownParam(obj check.Object) bool {
	rec, ok := r.fr.self.(*value.Record)
	decl, isParam := obj.Decl().(*syntax.Param)
	return ok && isParam && recordParam(recordOf(rec.T), decl) != nil
}

// recordParam is the parameter of rt declared by decl, nil when decl is not one of rt's.
func recordParam(rt *types.RecordType, decl *syntax.Param) *types.Param {
	if rt == nil || rt.Decl == nil {
		return nil
	}
	i := slices.Index(rt.Decl.Params, decl)
	for _, p := range rt.Params {
		if i >= 0 && p.Index == i {
			return p
		}
	}
	return nil
}

// fnArg is a type function's parameter p where cx gives its field a value, read along the first
// application reaching p's function: a record literal given to a dependent field is classified
// against the first branch that takes it, whose applied record reads p.
func (r *run) fnArg(cx *depCtx, p *types.Param) value.Value {
	root, paths, ok := types.ChainArg(cx.field, p)
	if !ok {
		return nil
	}
	v := r.argValue(root, cx)
	for _, path := range paths {
		if v == nil {
			return nil
		}
		v = r.follow(v, path, cx.at)
	}
	return v
}

// declared is the declared type cx gives a value, nil for none.
func (cx *depCtx) declared() types.Type {
	if cx == nil {
		return nil
	}
	return cx.field
}

// bound reports a dependent map's binder cx binds to a key.
func (cx *depCtx) bound(name string) bool {
	return cx != nil && cx.binders[name] != nil
}

// dependentKey reports a map key type whose names stay symbolic: a dependent type, maybe in a literal union (TYPES.md §11.4).
func dependentKey(t types.Type) bool {
	switch x := t.Base().(type) {
	case *types.DepUnionType, *types.TypeAppType:
		return true
	case *types.LitUnionType:
		return dependentKey(x.Of)
	}
	return false
}
