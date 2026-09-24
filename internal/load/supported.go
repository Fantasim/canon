package load

import "github.com/fantasim/canonlang/internal/types"

// supported reports whether t's decode never needs the evaluator: no ref, variant, map or
// dependent field, every default a plain literal of the field's own base kind (DECISIONS 173,
// meta/decisions/log-2026-09-24.md "load.dir review (M2)").
func supported(t types.Type) bool {
	switch b := t.Base().(type) {
	case *types.OptionalType:
		return supported(b.Elem)
	case *types.ListType:
		return supported(b.Elem)
	case *types.TableType:
		return supported(b.Elem)
	case *types.RecordType:
		return supportedFields(b.Fields)
	default:
		return scalarKind(b.Kind())
	}
}

// supportedFields is whether every field of a record supported reaches is itself supported.
func supportedFields(fields []*types.Field) bool {
	for _, f := range fields {
		if !supported(f.Type) {
			return false
		}
		if f.Default != nil && !defaultLiteralOK(f.Default, baseKind(f.Type)) {
			return false
		}
	}
	return true
}

// baseType is t with any Optional wrapper removed: what a present default value is typed as.
func baseType(t types.Type) types.Type {
	if opt, ok := t.Base().(*types.OptionalType); ok {
		return opt.Elem
	}
	return t
}

// baseKind is baseType(t)'s kind: the kind a field's literal default is checked against.
func baseKind(t types.Type) types.Kind {
	return baseType(t).Base().Kind()
}

// scalarKind is a kind wire decodes with no collection or reference of its own (WIRE.md §5.1).
func scalarKind(k types.Kind) bool {
	switch k {
	case types.Bool, types.Int, types.Float, types.String, types.Duration, types.Enum:
		return true
	default:
		return false
	}
}
