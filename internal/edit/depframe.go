package edit

import (
	"maps"
	"slices"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// depFrame is what a dependent type's arguments name (DEP-02): the record's fields, parameters and
// map binders, refs and defaults read through an applier's host; a strict frame (kept non-nil)
// refuses a name introduced into a Never branch (V1) and collects the held symbols it keeps.
type depFrame struct {
	rec     *value.Record
	params  map[*types.Param]value.Value
	binders map[string]value.Value
	a       *applier
	kept    map[*value.Symbol]bool
}

// branch is the type app selects in fr, and the frame binding its function's parameters (DEP-02):
// the arm its scrutinee selects, or its body. False when an argument or the scrutinee is unknown.
func (fr depFrame) branch(app *types.TypeAppType) (types.Type, depFrame, bool) {
	fn := app.Fn
	if len(fn.Params) != len(app.Args) {
		return nil, depFrame{}, false
	}
	inner := depFrame{params: make(map[*types.Param]value.Value, len(fn.Params)), a: fr.a, kept: fr.kept}
	for i, p := range fn.Params {
		v, ok := fr.arg(app.Args[i])
		if !ok {
			return nil, depFrame{}, false
		}
		inner.params[p] = v
	}
	s := fn.Scrutinee
	if s == nil {
		return fn.Body, inner, fn.Body != nil
	}
	v, ok := fr.follow(inner.params[s.Param], s.Path)
	if !ok {
		return nil, depFrame{}, false
	}
	i, ok := memberIndexOf(v)
	if !ok || fn.Arm(i) == nil {
		return nil, depFrame{}, false
	}
	return fn.Arm(i).Result, inner, true
}

// arg is an argument's value in fr: a path from an earlier field, a parameter or a binder.
func (fr depFrame) arg(a *types.Arg) (value.Value, bool) {
	var v value.Value
	path := a.Path
	switch a.Source {
	case types.ArgField:
		if len(path) == 0 {
			return nil, false
		}
		v, path = fr.field(path[0]), path[1:]
	case types.ArgParam:
		v = fr.params[a.Param]
	case types.ArgKey:
		v = fr.binders[a.Binder]
	}
	if v == nil {
		return nil, false
	}
	return fr.follow(v, path)
}

// field is f's value in fr's record: the value given, else with an applier its default (TYP-15),
// as the decoder reads a field left out; nil when there is none.
func (fr depFrame) field(f *types.Field) value.Value {
	if fr.rec == nil || f.Index >= len(fr.rec.Fields) {
		return nil
	}
	if v := fr.rec.Fields[f.Index]; v != nil || fr.a == nil {
		return v
	}
	v, ok := fr.a.defaultOf(fr.rec, f.Index)
	if !ok {
		return nil
	}
	return v
}

// follow is the value at a field path from v, through records and, with an applier, refs.
func (fr depFrame) follow(v value.Value, path []*types.Field) (value.Value, bool) {
	for _, f := range path {
		rec, ok := fr.record(v)
		if !ok || f.Index >= len(rec.Fields) {
			return nil, false
		}
		v = rec.Fields[f.Index]
	}
	return v, v != nil
}

// record is v as a record: itself, or with an applier the entry the ref v names.
func (fr depFrame) record(v value.Value) (*value.Record, bool) {
	switch x := v.(type) {
	case *value.Record:
		return x, x != nil
	case *value.Ref:
		if fr.a == nil {
			return nil, false
		}
		rec, ok := fr.a.host.Deref(fr.a.ctx, x)
		return rec, ok && rec != nil
	}
	return nil, false
}

// enter is the frame of r's fields, t r's declared type or nil: an applied record's parameters
// bound to its arguments read in fr.
func (fr depFrame) enter(r *value.Record, t types.Type) depFrame {
	in := depFrame{rec: r, a: fr.a, kept: fr.kept}
	ar, ok := baseOf(t).(*types.AppliedRecord)
	if !ok {
		ar, ok = r.T.Base().(*types.AppliedRecord)
	}
	if !ok || len(ar.Args) != len(ar.Rec.Params) {
		return in
	}
	in.params = make(map[*types.Param]value.Value, len(ar.Args))
	for i, p := range ar.Rec.Params {
		if v, found := fr.arg(ar.Args[i]); found {
			in.params[p] = v
		}
	}
	return in
}

// bind is fr with a dependent map's binder bound to one of its keys (TYPES.md §11.5).
func (fr depFrame) bind(binder string, key value.Value) depFrame {
	b := maps.Clone(fr.binders)
	if b == nil {
		b = map[string]value.Value{}
	}
	b[binder] = key
	return depFrame{rec: fr.rec, params: fr.params, binders: b, a: fr.a, kept: fr.kept}
}

// into is fr inside c, of declared type t or nil, reading its item child: a record's fields, a
// dependent map's value.
func (fr depFrame) into(c value.Value, t types.Type, child value.Value) depFrame {
	switch x := c.(type) {
	case *value.Record:
		return fr.enter(x, t)
	case *value.Map:
		dm, ok := declaredBase(t, x.T).(*types.DepMapType)
		i := slices.IndexFunc(x.Vals, func(v value.Value) bool { return v == child })
		if ok && i >= 0 {
			return fr.bind(dm.Binder, x.Keys[i])
		}
	}
	return fr
}
