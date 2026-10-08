package gogen

import (
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// isData reports a data- or types-mode emit: classes decoded from JSON, table ids strings, refs keys unless a load resolves them (CODEGEN.md §2.2, §5.3, §5.13).
func (g *gen) isData() bool { return g.e.Mode == ir.ModeData || g.isTypes() }

// isTypes reports a types-mode emit: public decoders of the source wire, no value (CODEGEN.md §5.13).
func (g *gen) isTypes() bool { return g.e.Mode == ir.ModeTypes }

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
	u := g.names.Foreign()
	for _, class := range u.Built() {
		if c, ok := class.(*ir.Case); ok && u.VariantOf(c) != nil {
			g.variantOf[c] = u.VariantOf(c)
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
	case (t.Kind == types.List || t.Kind == types.Table || t.Kind == types.Map) && t.Elem != nil:
		return g.typeWalks(*t.Elem)
	}
	return false
}

// valueWalks reports a type a resolver walks: one holding, by value, a class it resolves, or a map whose ref keys it checks (WIRE.md §5.8).
func (g *gen) valueWalks(t ir.TypeRef) bool {
	switch {
	case t.Kind == types.Record, t.Kind == types.Variant:
		return g.names.NeedsWalk(t.Named)
	case t.Kind == types.Map && g.names.MapKeyTarget(t, g.walkClass) != nil:
		return true
	case (t.Kind == types.List || t.Kind == types.Table || t.Kind == types.Map) && t.Elem != nil:
		return g.valueWalks(*t.Elem)
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
