package std

import (
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// Elems are the elements of a sequence: a list's elements, a table's entries in entry order.
func Elems(v value.Value) []value.Value {
	switch x := v.(type) {
	case *value.List:
		return x.Elems
	case *value.Table:
		out := make([]value.Value, len(x.Entries))
		for i, e := range x.Entries {
			out[i] = e
		}
		return out
	}
	return nil
}

// KeyOf is the key a value names in a keyed collection (TYPES.md §9, STDLIB.md §5).
func KeyOf(v value.Value) (value.Key, bool) {
	switch x := v.(type) {
	case *value.Str:
		return value.Key{S: x.V}, true
	case *value.Int:
		return value.Key{I: x.V, IsInt: true}, true
	case *value.Member:
		return value.Key{S: x.Enum.Members[x.Index].Name}, true
	case *value.Ref:
		return x.Key, true
	case *value.Record:
		if x.Ident != nil {
			return x.Ident.Key, true
		}
	}
	return value.Key{}, false
}

func (c *Call) intv(n int64) value.Value {
	return &value.Int{V: n, T: types.IntType, P: c.Prov}
}

func (c *Call) boolv(b bool) value.Value {
	return &value.Bool{V: b, P: c.Prov}
}

func (c *Call) strv(s string) value.Value {
	return &value.Str{V: s, T: types.StringType, P: c.Prov}
}

func (c *Call) none() value.Value {
	return &value.None{T: c.Result, P: c.Prov}
}

// list is a plain list of the call's result type.
func (c *Call) list(xs []value.Value) value.Value {
	return &value.List{T: c.Result, Elems: xs, P: c.Prov}
}

// orNone is x, or none when absent.
func (c *Call) orNone(x value.Value, ok bool) value.Value {
	if !ok {
		return c.none()
	}
	return x
}

// elemType is the element type of a list type, the key or value type of a pair (A, B).
func elemType(t types.Type) types.Type {
	if l, ok := t.Base().(*types.ListType); ok {
		return l.Elem
	}
	return types.AnyType
}

func pairOf(t types.Type, a, b value.Value, p *value.Prov) value.Value {
	return &value.Pair{T: t, A: a, B: b, P: p}
}

// arg is argument i of the call.
func (c *Call) arg(i int) value.Value {
	return c.Args[i]
}

// holds invokes a predicate on args: its Bool result.
func holds(h Host, fn value.Value, args ...value.Value) (bool, bool) {
	v, ok := h.Invoke(fn, args...)
	if !ok {
		return false, false
	}
	b, isBool := v.(*value.Bool)
	return isBool && b.V, isBool
}
