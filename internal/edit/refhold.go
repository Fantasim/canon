package edit

import "github.com/fantasim/canonlang/internal/types"

// mayHold reports that a value of type t could hold the target, by t alone: a ref to its
// collection, the member's enum, or a type computed from a value; a broken let, typed Error, has
// no value to hold one (log-2026-09-29 M4 U4a).
func (tg *target) mayHold(t types.Type) bool {
	return typeHolds(t, tg.leaf, map[types.Type]bool{})
}

// missing is why a let has no value, when it could hold the target; nil for one that cannot.
func (tg *target) missing(r rootRef, err error) error {
	if tg.mayHold(r.obj.Type()) {
		return err
	}
	return nil
}

// leaf decides a base type that holds the target or not by itself; decided false walks its components.
func (tg *target) leaf(b types.Type) (holds, decided bool) {
	switch b.Kind() {
	case types.TypeApp, types.DepUnion, types.Any:
		return true, true
	case types.Ref:
		r, ok := b.(*types.RefType)
		return ok && tg.names(r.Target), true
	case types.Enum:
		return tg.member != nil && b == types.Type(tg.member.Enum), true
	default:
	}
	return false, false
}

// typeHolds walks t's components once each until leaf decides one holds; seen breaks the
// cycles of recursive records.
func typeHolds(t types.Type, leaf func(types.Type) (holds, decided bool), seen map[types.Type]bool) bool {
	if t == nil || seen[t.Base()] {
		return false
	}
	b := t.Base()
	seen[b] = true
	if holds, decided := leaf(b); decided {
		return holds
	}
	for _, c := range components(b) {
		if typeHolds(c, leaf, seen) {
			return true
		}
	}
	return false
}

// components are the types a value of t is made of: elements, keys and values, fields, cases.
func components(t types.Type) []types.Type {
	switch b := t.(type) {
	case *types.OptionalType:
		return []types.Type{b.Elem}
	case *types.ListType:
		return []types.Type{b.Elem}
	case *types.TableType:
		return []types.Type{b.Elem}
	case *types.MapType:
		return []types.Type{b.Key, b.Value}
	case *types.DepMapType:
		return []types.Type{&types.RefType{Target: b.Coll}, b.Value}
	case *types.LitUnionType:
		return []types.Type{b.Of}
	case *types.PairType:
		return []types.Type{b.A, b.B}
	case *types.VariantType:
		out := make([]types.Type, len(b.Cases))
		for i, c := range b.Cases {
			out[i] = c
		}
		return out
	}
	return fieldTypes(fieldsOf(t))
}

func fieldTypes(fields []*types.Field) []types.Type {
	out := make([]types.Type, len(fields))
	for i, f := range fields {
		out[i] = f.Type
	}
	return out
}
