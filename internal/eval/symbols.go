package eval

import (
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// branchBase is t without its optional and literal-union layers: the type a symbol names in.
func branchBase(t types.Type) types.Type {
	for {
		switch x := t.Base().(type) {
		case *types.OptionalType:
			t = x.Elem
		case *types.LitUnionType:
			t = x.Of
		default:
			return x
		}
	}
}

// named reports a type a symbol may name something in: an enum, a ref target or a variant.
func named(t types.Type) bool {
	switch branchBase(t).(type) {
	case *types.EnumType, *types.RefType, *types.VariantType, *types.CaseType:
		return true
	}
	return false
}

// symbolAs is s as a member, key or defaulted case of t, s itself when t names none (TYPES.md §11.6).
func (r *run) symbolAs(s *value.Symbol, t types.Type) value.Value {
	v, fit := ToBranch(s, branchBase(t), true)
	switch fit {
	case Fits:
		return v
	case BareCase:
		rec := v.(*value.Record)
		if f, ok := r.bare(rec); ok && f == nil {
			return rec
		}
	case NoFit:
	}
	return s
}

// valueAs is x, a symbol, as a value of t, a container's computed element or key type (TYPES.md §7.5).
func (r *run) valueAs(x value.Value, t types.Type) value.Value {
	s, ok := x.(*value.Symbol)
	if !ok || t == nil || r.ev.dependent(t) || !named(t) {
		return x
	}
	return r.symbolAs(s, t)
}

// keyFor is k as a key of m, converted against m's computed key type in O(1) (DECISIONS 199).
func (r *run) keyFor(m *value.Map, k value.Value) value.Value {
	return r.valueAs(k, mapKeyType(m.T))
}

// elemOf is the element type of a list type, nil for another.
func elemOf(t types.Type) types.Type {
	if t == nil {
		return nil
	}
	if l, ok := t.Base().(*types.ListType); ok {
		return l.Elem
	}
	return nil
}

// settledList is l, a list built from lists, typed as the first of them whose element type
// verification computed, its symbols converted to it; l itself when none has one.
func (r *run) settledList(l *value.List, from ...value.Value) *value.List {
	if l == nil || !r.ev.dependent(elemOf(l.T)) {
		return l
	}
	for _, f := range from {
		fl, ok := f.(*value.List)
		if !ok || fl.T == nil || r.ev.dependent(elemOf(fl.T)) {
			continue
		}
		elems := make([]value.Value, len(l.Elems))
		for i, e := range l.Elems {
			elems[i] = r.valueAs(e, elemOf(fl.T))
		}
		return &value.List{T: fl.T, Elems: elems, P: l.P}
	}
	return l
}
