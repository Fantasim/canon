package eval

import (
	"slices"

	"github.com/fantasim/canonlang/internal/types"
)

// mayHoldRef reports whether a value of type t may hold a ref: a binder need not walk one
// that cannot, so rebinding an amended instance costs its ref-holding parts only.
func (e *Evaluator) mayHoldRef(t types.Type) bool {
	if h, ok := e.refTypes[t]; ok {
		return h
	}
	h := holdsRef(t, map[types.Type]bool{})
	e.refTypes[t] = h
	return h
}

// holdsRef is mayHoldRef without memo; a type already on the way adds no ref.
func holdsRef(t types.Type, seen map[types.Type]bool) bool {
	if t == nil {
		return true
	}
	if seen[t] {
		return false
	}
	seen[t] = true
	parts, known := typeParts(t)
	return !known || slices.ContainsFunc(parts, func(p types.Type) bool { return holdsRef(p, seen) })
}

// typeParts are the types a value of t holds; a ref, or a type it does not know, may be one.
func typeParts(t types.Type) ([]types.Type, bool) {
	switch x := t.(type) {
	case types.Basic, *types.EnumType, *types.VariantKindType:
		return nil, true
	case *types.Alias:
		return []types.Type{x.Def}, true
	case *types.Refined:
		return []types.Type{x.Of}, true
	case *types.OptionalType:
		return []types.Type{x.Elem}, true
	case *types.LitUnionType:
		return []types.Type{x.Of}, true
	case *types.ListType:
		return []types.Type{x.Elem}, true
	case *types.TableType:
		return []types.Type{x.Elem}, true
	case *types.MapType:
		return []types.Type{x.Key, x.Value}, true
	case *types.PairType:
		return []types.Type{x.A, x.B}, true
	case *types.RecordType:
		return fieldTypes(x.Fields), true
	case *types.CaseType:
		return fieldTypes(x.Fields), true
	case *types.VariantType:
		out := make([]types.Type, len(x.Cases))
		for i, c := range x.Cases {
			out[i] = c
		}
		return out, true
	}
	return nil, false
}

func fieldTypes(fields []*types.Field) []types.Type {
	out := make([]types.Type, len(fields))
	for i, f := range fields {
		out[i] = f.Type
	}
	return out
}
