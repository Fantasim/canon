package verify

import (
	"maps"
	"slices"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// env is what a type argument names where a value sits (TYPES.md §11.1).
type env struct {
	rec     *value.Record                // the record whose earlier fields an argument reads
	app     *types.AppliedRecord         // rec's application, whose arguments are read in up
	up      *env                         // the env around rec, or a type function's caller
	params  map[*types.Param]value.Value // a type function's arguments, or app's once read
	binders map[string]value.Value       // the dependent map binders in scope
}

// within is the env of rec's fields, rec applied as its declared type t or as its own.
func (e *env) within(rec *value.Record, t types.Type) *env {
	out := &env{rec: rec, up: e}
	if a, ok := appliedOf(t); ok {
		out.app = a
	} else if a, ok := appliedOf(rec.T); ok {
		out.app = a
	}
	return out
}

// bind is e with a dependent map's binder bound to one of its keys (TYPES.md §11.5).
func (e *env) bind(binder string, key value.Value) *env {
	out := &env{}
	if e != nil {
		*out = *e
	}
	out.binders = maps.Clone(out.binders)
	if out.binders == nil {
		out.binders = map[string]value.Value{}
	}
	out.binders[binder] = key
	return out
}

// owner is the nearest record around e whose type holds c, a collection field (TYPES.md §10.2).
func (e *env) owner(c *types.Collection) *value.Record {
	for x := e; x != nil; x = x.up {
		if x.rec != nil && recordOf(x.rec.T) == c.Owner {
			return x.rec
		}
	}
	return nil
}

func appliedOf(t types.Type) (*types.AppliedRecord, bool) {
	if t == nil {
		return nil, false
	}
	a, ok := t.Base().(*types.AppliedRecord)
	return a, ok
}

// recordOf is the record type of a record or applied record type, nil for another.
func recordOf(t types.Type) *types.RecordType {
	if t == nil {
		return nil
	}
	switch r := t.Base().(type) {
	case *types.RecordType:
		return r
	case *types.AppliedRecord:
		return r.Rec
	}
	return nil
}

// read is how reading a type argument ended.
type read uint8

// arg is an argument's value in e, a path from a parameter, an earlier field or a binder (TYPES.md §11.1).
func (w *walker) arg(a *types.Arg, e *env) (value.Value, read) {
	if e == nil {
		return nil, missing
	}
	var v value.Value
	path := a.Path
	switch a.Source {
	case types.ArgParam:
		p, r := w.param(a.Param, e)
		if r != found {
			return nil, r
		}
		v = p
	case types.ArgKey:
		v = e.binders[a.Binder]
	default:
		if len(path) == 0 || e.rec == nil || path[0].Index >= len(e.rec.Fields) {
			return nil, missing
		}
		v, path = e.rec.Fields[path[0].Index], path[1:]
	}
	return w.follow(v, path)
}

// param is p's argument in e: bound, or read once from e's application in the env around it.
func (w *walker) param(p *types.Param, e *env) (value.Value, read) {
	if v, ok := e.params[p]; ok {
		return v, found
	}
	if e.app == nil || p.Index >= len(e.app.Args) || !slices.Contains(e.app.Rec.Params, p) {
		return nil, missing
	}
	v, r := w.arg(e.app.Args[p.Index], e.up)
	if r != found {
		return nil, r
	}
	if e.params == nil {
		e.params = map[*types.Param]value.Value{}
	}
	e.params[p] = v
	return v, found
}

// follow reads fields along path, dereferencing refs.
func (w *walker) follow(v value.Value, path []*types.Field) (value.Value, read) {
	for _, f := range path {
		rec, r := w.deref(v)
		if r != found {
			return nil, r
		}
		if f.Index >= len(rec.Fields) {
			return nil, missing
		}
		v = rec.Fields[f.Index]
	}
	if v == nil {
		return nil, missing
	}
	return v, found
}

// deref is the record v is or the entry it names; a ref to none aborts the reader (EVALUATION.md §7.3).
func (w *walker) deref(v value.Value) (*value.Record, read) {
	switch x := v.(type) {
	case *value.Record:
		return x, found
	case *value.Ref:
		rt, ok := x.T.Base().(*types.RefType)
		if !ok || rt.Target == nil {
			return nil, missing
		}
		entries, how := w.collection(x, rt.Target)
		if e := entries[x.Key]; how == reached && e != nil {
			return e, found
		}
		return nil, aborted
	}
	return nil, missing
}
