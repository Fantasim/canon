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
// is reported, never nil.
type depCtx struct {
	rec     *value.Record
	params  map[*types.Param]value.Value
	binders map[string]value.Value
	field   types.Type
	at      syntax.Node
}

// withBinder is cx with a dependent map's binder bound to one of its keys (TYPES.md §11.5).
func (cx *depCtx) withBinder(name string, key value.Value, at syntax.Node) *depCtx {
	out := &depCtx{binders: map[string]value.Value{}, at: at}
	if cx != nil {
		out.rec, out.params, out.field = cx.rec, cx.params, cx.field
		maps.Copy(out.binders, cx.binders)
	}
	out.binders[name] = key
	return out
}

// forField is cx giving a value to a field of declared type t.
func (cx *depCtx) forField(t types.Type) *depCtx {
	out := *cx
	out.field = t
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
	chain := appChain(cx.field, p, map[*types.TypeFunc]bool{})
	if len(chain) == 0 {
		return nil
	}
	return r.chainArg(chain, p, cx)
}

// chainArg is p's argument in the last application of chain, each application's arguments
// naming the parameters of the one before it; only the arguments on the way are read.
func (r *run) chainArg(chain []*types.TypeAppType, p *types.Param, cx *depCtx) value.Value {
	app := chain[len(chain)-1]
	i := slices.Index(app.Fn.Params, p)
	if i < 0 || i >= len(app.Args) {
		return nil
	}
	a := app.Args[i]
	if len(chain) == 1 {
		return r.argValue(a, cx)
	}
	if a.Source != types.ArgParam {
		return nil
	}
	if root := r.chainArg(chain[:len(chain)-1], a.Param, cx); root != nil {
		return r.follow(root, a.Path, cx.at)
	}
	return nil
}

// appChain is the applications from t down to one of p's function, outermost first, each
// function's body then arms searched once; nil when none reaches it.
func appChain(t types.Type, p *types.Param, seen map[*types.TypeFunc]bool) []*types.TypeAppType {
	if t == nil {
		return nil
	}
	switch x := t.Base().(type) {
	case *types.OptionalType:
		return appChain(x.Elem, p, seen)
	case *types.ListType:
		return appChain(x.Elem, p, seen)
	case *types.MapType:
		return appChain(x.Value, p, seen)
	case *types.DepMapType:
		return appChain(x.Value, p, seen)
	case *types.LitUnionType:
		return appChain(x.Of, p, seen)
	case *types.TypeAppType:
		return fnChain(x, p, seen)
	}
	return nil
}

func fnChain(app *types.TypeAppType, p *types.Param, seen map[*types.TypeFunc]bool) []*types.TypeAppType {
	fn := app.Fn
	if seen[fn] {
		return nil
	}
	seen[fn] = true
	if slices.Contains(fn.Params, p) {
		return []*types.TypeAppType{app}
	}
	results := []types.Type{fn.Body}
	for _, arm := range fn.Arms {
		results = append(results, arm.Result)
	}
	for _, res := range results {
		if rest := appChain(res, p, seen); rest != nil {
			return append([]*types.TypeAppType{app}, rest...)
		}
	}
	return nil
}

// binderOf is the binder of a dependent map type, or of a map whose values apply one (TYPES.md §11.5).
func binderOf(t types.Type) string {
	switch x := t.Base().(type) {
	case *types.DepMapType:
		return x.Binder
	case *types.MapType:
		return argBinder(x.Value)
	}
	return ""
}

// mapBinder is the binder a map literal of type t binds to each key: where cx gives a declared
// type, that of its dependent map on the same collection, which a static view erases; else
// binderOf's.
func mapBinder(t types.Type, cx *depCtx) string {
	mt, ok := t.Base().(*types.MapType)
	if !ok || cx == nil || cx.field == nil {
		return binderOf(t)
	}
	if rt, isRef := mt.Key.Base().(*types.RefType); isRef {
		if d := depMapOn(cx.field, rt.Target); d != nil {
			return d.Binder
		}
	}
	return ""
}

// depMapOn is the first dependent map keyed by coll in t, through optionals, lists and map values.
func depMapOn(t types.Type, coll *types.Collection) *types.DepMapType {
	switch x := t.Base().(type) {
	case *types.OptionalType:
		return depMapOn(x.Elem, coll)
	case *types.ListType:
		return depMapOn(x.Elem, coll)
	case *types.MapType:
		return depMapOn(x.Value, coll)
	case *types.DepMapType:
		if x.Coll == coll {
			return x
		}
		return depMapOn(x.Value, coll)
	}
	return nil
}

// argBinder is the binder an argument of an application in t reads, "" for none.
func argBinder(t types.Type) string {
	var args []*types.Arg
	switch x := t.Base().(type) {
	case *types.OptionalType:
		return argBinder(x.Elem)
	case *types.ListType:
		return argBinder(x.Elem)
	case *types.AppliedRecord:
		args = x.Args
	case *types.TypeAppType:
		args = x.Args
	}
	for _, a := range args {
		if a.Source == types.ArgKey {
			return a.Binder
		}
	}
	return ""
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
