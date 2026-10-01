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
	res, _, ok := depFrame{rec: rec}.branch(app)
	if !ok || dependent(res) {
		return nil, false
	}
	if isOptional(t) {
		return &types.OptionalType{Elem: res}, true
	}
	return res, true
}

// computedType is t with its type applications computed in fr (DEP-02): the arm each scrutinee
// selects, through arms that are applications themselves, an optional layer kept. False when an
// argument or a scrutinee is unknown here.
func (fr depFrame) computedType(t types.Type) (types.Type, bool) {
	opt := false
	for range maxLitDepth {
		opt = opt || isOptional(t)
		t = present(t)
		app, isApp := t.Base().(*types.TypeAppType)
		if !isApp {
			break
		}
		br, inner, ok := fr.branch(app)
		if !ok {
			return nil, false
		}
		t, fr = br, inner
	}
	switch {
	case dependent(t):
		return nil, false
	case opt:
		return &types.OptionalType{Elem: t}, true
	}
	return t, true
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
