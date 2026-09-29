package edit

import (
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// concreteType is t, a field's type in rec, with a type application computed from rec's fields:
// the arm its scrutinee selects, or its body. False when an argument is no field of rec, a field
// on the way is not a record, or the arm is itself computed.
func concreteType(t types.Type, rec *value.Record) (types.Type, bool) {
	app, ok := present(t).Base().(*types.TypeAppType)
	if !ok {
		return t, true
	}
	fn := app.Fn
	if len(fn.Params) != len(app.Args) {
		return nil, false
	}
	params := map[*types.Param]value.Value{}
	for i, p := range fn.Params {
		arg := app.Args[i]
		v, found := followFields(rec, arg.Path)
		if arg.Source != types.ArgField || !found {
			return nil, false
		}
		params[p] = v
	}
	res, ok := armOf(fn, params)
	if !ok || dependent(res) {
		return nil, false
	}
	if isOptional(t) {
		return &types.OptionalType{Elem: res}, true
	}
	return res, true
}

// armOf is the type fn's scrutinee selects among its arms, or its body.
func armOf(fn *types.TypeFunc, params map[*types.Param]value.Value) (types.Type, bool) {
	s := fn.Scrutinee
	if s == nil {
		return fn.Body, fn.Body != nil
	}
	v, found := followFields(params[s.Param], s.Path)
	if !found {
		return nil, false
	}
	i, ok := memberIndexOf(v)
	if !ok || fn.Arm(i) == nil {
		return nil, false
	}
	return fn.Arm(i).Result, true
}

// followFields is the value at a field path from v, through records.
func followFields(v value.Value, path []*types.Field) (value.Value, bool) {
	for _, f := range path {
		rec, ok := v.(*value.Record)
		if !ok || f.Index >= len(rec.Fields) {
			return nil, false
		}
		v = rec.Fields[f.Index]
	}
	return v, v != nil
}

// memberIndexOf is the arm index a scrutinee value selects: a member's, a case's, a Bool's.
func memberIndexOf(v value.Value) (int, bool) {
	switch x := v.(type) {
	case *value.Member:
		return x.Index, true
	case *value.CaseKind:
		return x.Index, true
	case *value.Bool:
		if x.V {
			return 1, true
		}
		return 0, true
	}
	return 0, false
}
