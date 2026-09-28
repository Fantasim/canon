package verify

import (
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// substitute is t with each application it holds replaced by the type it computes in e (TYPES.md §11.6).
func (w *walker) substitute(t types.Type, e *env) types.Type {
	switch x := t.(type) {
	case *types.TypeAppType:
		if bt, inner, ok := w.branch(x, e); ok {
			return w.substitute(bt, inner)
		}
	case *types.OptionalType:
		if elem := w.substitute(x.Elem, e); elem != x.Elem {
			return &types.OptionalType{Elem: elem}
		}
	case *types.LitUnionType:
		if of := w.substitute(x.Of, e); of != x.Of {
			return &types.LitUnionType{Of: of, Literals: x.Literals}
		}
	case *types.ListType:
		if elem := w.substitute(x.Elem, e); elem != x.Elem {
			return &types.ListType{Elem: elem, KeyedBy: x.KeyedBy}
		}
	case *types.MapType:
		return w.substituteMap(x, e)
	}
	return t
}

func (w *walker) substituteMap(m *types.MapType, e *env) types.Type {
	k, v := w.substitute(m.Key, e), w.substitute(m.Value, e)
	if k == m.Key && v == m.Value {
		return m
	}
	return &types.MapType{Key: k, Value: v}
}

// holdsApp reports a list or map type whose elements, keys or values are applications.
func holdsApp(t types.Type) bool {
	switch x := t.(type) {
	case *types.TypeAppType:
		return true
	case *types.OptionalType:
		return holdsApp(x.Elem)
	case *types.LitUnionType:
		return holdsApp(x.Of)
	case *types.ListType:
		return holdsApp(x.Elem)
	case *types.MapType:
		return holdsApp(x.Key) || holdsApp(x.Value)
	}
	return false
}

// retyped is v, a list, map or pair, typed as t, its computed type (TYPES.md §11.6, §7.5).
func (w *walker) retyped(v value.Value, t types.Type) value.Value {
	var nv value.Value
	switch x := v.(type) {
	case *value.List:
		if x.T == t {
			return v
		}
		nv = &value.List{T: t, Elems: x.Elems, P: x.P}
	case *value.Map:
		if x.T == t {
			return v
		}
		nv = &value.Map{T: t, Keys: x.Keys, Vals: x.Vals, P: x.P}
	case *value.Pair:
		if x.T == t {
			return v
		}
		nv = &value.Pair{T: t, A: x.A, B: x.B, P: x.P}
	default:
		return v
	}
	return w.moved(v, nv)
}

// moved is to, the copy verification built of from, carrying its marks and a record's collections.
func (w *walker) moved(from, to value.Value) value.Value {
	if w.stage != nil && from != to {
		w.stage.Moved(from, to)
	}
	return to
}
