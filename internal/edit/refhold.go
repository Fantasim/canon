package edit

import "github.com/fantasim/canonlang/internal/types"

// mayHold reports that a value of type t could hold the target, by t alone: a ref to its
// collection, the member's enum, or a type computed from a value; a broken let, typed Error, has
// no value to hold one (log-2026-09-29 M4 U4a).
func (tg *target) mayHold(t types.Type) bool {
	return tg.holds(t, map[types.Type]bool{})
}

// missing is why a let has no value, when it could hold the target; nil for one that cannot.
func (tg *target) missing(r rootRef, err error) error {
	if tg.mayHold(r.obj.Type()) {
		return err
	}
	return nil
}

// holds walks t's components once each; seen breaks the cycles of recursive records.
func (tg *target) holds(t types.Type, seen map[types.Type]bool) bool {
	if t == nil || seen[t.Base()] {
		return false
	}
	b := t.Base()
	seen[b] = true
	switch b.Kind() {
	case types.TypeApp, types.DepUnion, types.Any:
		return true
	case types.Ref:
		r, ok := b.(*types.RefType)
		return ok && tg.ident != nil && r.Target == tg.ident.Coll
	case types.Enum:
		return tg.member != nil && b == types.Type(tg.member.Enum)
	default:
	}
	for _, c := range components(b) {
		if tg.holds(c, seen) {
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
