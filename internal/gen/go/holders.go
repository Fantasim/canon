package gogen

import (
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// isData reports a data-mode emit (CODEGEN.md §2.2).
func (g *gen) isData() bool { return g.e.Mode == ir.ModeData }

// indexVariants maps each case with fields to its variant (WIRE.md §5.6, CODEGEN.md §5.5).
func (g *gen) indexVariants() {
	g.variantOf = map[*ir.Case]*ir.Variant{}
	for _, t := range g.p.Types {
		if v, ok := t.(*ir.Variant); ok {
			for _, c := range v.Cases {
				g.variantOf[c] = v
			}
		}
	}
}

// slotTarget is the value a resolved slot's ref points into, by the ref's own value name (CODEGEN.md §5.8).
func (g *gen) slotTarget(s *slot) *ir.Value {
	if s.Ref == nil || !s.Resolved {
		return nil
	}
	return g.byValue[s.Ref.Value].v
}

// typeWalks reports a type holding, by value, a class the plan resolves at load (CODEGEN.md §5.8).
func (g *gen) typeWalks(t ir.TypeRef) bool {
	switch {
	case t.Kind == types.Record, t.Kind == types.Variant:
		return g.names.NeedsWalk(t.Named)
	case (t.Kind == types.List || t.Kind == types.Table) && t.Elem != nil:
		return g.typeWalks(*t.Elem)
	}
	return false
}

// rootKey is the class of a value's rows (a table or keyed list) or of the value itself.
func rootKey(v *ir.Value) any {
	t := v.Type
	if t.Kind != types.Record && t.Elem != nil {
		t = *t.Elem
	}
	if t.Kind == types.Record && t.Named != nil {
		return t.Named
	}
	return nil
}
