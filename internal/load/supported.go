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

// hasDependent reports t or anything reachable from it needing branch selection (DECISIONS 173).
func hasDependent(t types.Type) bool {
	return (&dependentScan{seen: map[types.Type]bool{}}).visit(t)
}

// unsupportedDependent is bare load's and load.csv's cause for a dependent element type.
func unsupportedDependent() error {
	return unsupported(causeDependent)
}

// dependentScan finds a TypeAppType reachable from a root type, every type visited once.
type dependentScan struct {
	seen map[types.Type]bool
}

func (s *dependentScan) visit(t types.Type) bool {
	if t == nil || s.seen[t] {
		return false
	}
	s.seen[t] = true
	switch b := t.Base().(type) {
	case *types.TypeAppType:
		return true
	case *types.OptionalType:
		return s.visit(b.Elem)
	case *types.ListType:
		return s.visit(b.Elem)
	case *types.TableType:
		return s.visit(b.Elem)
	case *types.MapType:
		return s.visit(b.Key) || s.visit(b.Value)
	case *types.DepMapType:
		return s.visit(b.Value)
	case *types.RecordType:
		return s.fields(b.Fields)
	case *types.AppliedRecord:
		return s.fields(b.Rec.Fields)
	case *types.VariantType:
		for _, c := range b.Cases {
			if s.fields(c.Fields) {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func (s *dependentScan) fields(fields []*types.Field) bool {
	for _, f := range fields {
		if s.visit(f.Type) {
			return true
		}
	}
	return false
}

// hasBadDefault reports a field, anywhere reachable from t, whose default is not a plain literal (WIRE.md §6.1).
func hasBadDefault(t types.Type) bool {
	return (&defaultScan{seen: map[types.Type]bool{}}).visit(t)
}

// unsupportedDefault is bare load's and load.csv's cause for a default value the host cannot read.
func unsupportedDefault() error {
	return unsupported(causeDefault)
}

// defaultScan finds a bad field default reachable from a root type, every type visited once.
type defaultScan struct {
	seen map[types.Type]bool
}

func (s *defaultScan) visit(t types.Type) bool {
	if t == nil || s.seen[t] {
		return false
	}
	s.seen[t] = true
	switch b := t.Base().(type) {
	case *types.OptionalType:
		return s.visit(b.Elem)
	case *types.ListType:
		return s.visit(b.Elem)
	case *types.TableType:
		return s.visit(b.Elem)
	case *types.MapType:
		return s.visit(b.Key) || s.visit(b.Value)
	case *types.DepMapType:
		return s.visit(b.Value)
	case *types.RecordType:
		return s.fields(b.Fields)
	case *types.AppliedRecord:
		return s.fields(b.Rec.Fields)
	case *types.VariantType:
		for _, c := range b.Cases {
			if s.fields(c.Fields) {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func (s *defaultScan) fields(fields []*types.Field) bool {
	for _, f := range fields {
		if f.Default != nil && !defaultLiteralOK(f.Default, baseKind(f.Type)) {
			return true
		}
		if s.visit(f.Type) {
			return true
		}
	}
	return false
}
