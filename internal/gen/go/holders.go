package gogen

import (
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// isData reports a data-mode emit (CODEGEN.md §2.2).
func (g *gen) isData() bool { return g.e.Mode == ir.ModeData }

// indexHolders lists, per record, variant and case, the emitted values holding it by value,
// directly or through fields, lists and stored fn results: the classes a loader decodes.
func (g *gen) indexHolders() {
	g.holders, g.variantOf = map[any][]*ir.Value{}, map[*ir.Case]*ir.Variant{}
	for _, t := range g.p.Types {
		if v, ok := t.(*ir.Variant); ok {
			for _, c := range v.Cases {
				g.variantOf[c] = v
			}
		}
	}
	g.decoded = map[any]bool{}
	for _, v := range g.emitted {
		if key := rootKey(v); key != nil {
			g.hold(key, v, map[any]bool{})
			g.decodes(key)
		}
	}
}

// decodes marks the classes a loader decodes whole: a pairs record is read slot by slot.
func (g *gen) decodes(key any) {
	if g.decoded[key] {
		return
	}
	g.decoded[key] = true
	for _, to := range heldBy(key, false) {
		g.decodes(to)
	}
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

func (g *gen) hold(key any, v *ir.Value, seen map[any]bool) {
	if seen[key] {
		return
	}
	seen[key] = true
	g.holders[key] = append(g.holders[key], v)
	for _, to := range heldBy(key, true) {
		g.hold(to, v, seen)
	}
}

// heldBy are the classes a class holds by value: a variant its cases with fields, a record or
// case those of its fields, pairs fields when pairs is set, and of its stored fns' results.
func heldBy(key any, pairs bool) []any {
	var fields []*ir.Field
	var fns []*ir.ExportFn
	switch k := key.(type) {
	case *ir.Record:
		fields, fns = k.Fields, k.Methods
	case *ir.Case:
		fields, fns = k.Fields, k.Methods
	case *ir.Variant:
		var out []any
		for _, c := range k.Cases {
			if len(c.Fields) > 0 {
				out = append(out, c)
			}
		}
		return out
	}
	var out []any
	for _, f := range fields {
		if pairs || f.Pairs == nil {
			out = classesOf(f.Type, out)
		}
	}
	for _, fn := range fns {
		if fn.Kind != ir.FnTranslated {
			out = classesOf(fn.Result, out)
		}
	}
	return out
}

// classesOf adds the record or variant t is, or its element's, through lists and optionals.
func classesOf(t ir.TypeRef, out []any) []any {
	if (t.Kind == types.Record || t.Kind == types.Variant) && t.Named != nil {
		return append(out, t.Named)
	}
	if t.Elem != nil && t.Kind != types.Map {
		return classesOf(*t.Elem, out)
	}
	return out
}

// dataSlot lays out a data-mode ref: its key always, its entry when resolved, ok when optional (CODEGEN.md §5.8).
func (g *gen) dataSlot(s *slot, key any) {
	if !g.isData() || s.Ref == nil {
		return
	}
	s.Resolved = g.resolvesTo(s.Ref, key, s.origin) != nil
	s.Main, s.Key, s.OK = s.Resolved, true, s.Optional
}

// dataFinite refuses a data-mode lookup over a parameter that is no enum or Bool; a ref result keeps a table of keys beside its entries (CODEGEN.md §5.10).
func (g *gen) dataFinite(f *finiteMethod, key any) {
	if !g.isData() {
		return
	}
	for _, p := range f.fn.Params {
		if p.Type.Kind != types.Bool && p.Type.Kind != types.Enum {
			g.fail(newDetail(ErrUnsupported, f.origin, lookupParamFormat, f.origin))
		}
	}
	g.dataSlot(f.res, key)
	g.setPair(f)
}

// resolvesTo is what a data-mode ref of class key resolves into: its holder's own value, or any @reload value when the holder is one; nil else, refused under several holders (CODEGEN.md §5.8, §5.11).
func (g *gen) resolvesTo(r *ir.RefTarget, key any, origin string) *ir.Value {
	if r == nil || r.Coll != types.CollLet || r.Local || r.Pkg != g.p.Name || g.byValue[r.Value] == nil {
		return nil
	}
	target := g.byValue[r.Value].v
	if !isContainer(target) {
		return nil
	}
	holders := g.holders[key]
	for _, w := range holders {
		if w != target && (!w.Reload || !target.Reload) {
			continue
		}
		if len(holders) > 1 {
			g.fail(newDetail(ErrUnsupported, origin, severalHoldersFormat, origin))
			return nil
		}
		return target
	}
	return nil
}

// classBody is the body of a record or case class, nil for a variant.
func (g *gen) classBody(key any) *body {
	switch k := key.(type) {
	case *ir.Record:
		return g.bodyOf(k)
	case *ir.Case:
		if v := g.variantOf[k]; v != nil {
			return g.caseBody(v, k)
		}
	}
	return nil
}

// needsWalk reports a class holding a resolved ref, itself or through a class it holds.
func (g *gen) needsWalk(key any) bool { return g.reaches(key, map[any]bool{}) }

func (g *gen) reaches(key any, seen map[any]bool) bool {
	if seen[key] || g.holders[key] == nil {
		return false
	}
	seen[key] = true
	if b := g.classBody(key); b != nil {
		for _, s := range b.slots {
			if s.Resolved {
				return true
			}
		}
	}
	for _, to := range heldBy(key, true) {
		if g.reaches(to, seen) {
			return true
		}
	}
	return false
}

// typeWalks reports a type holding, by value, a class to resolve.
func (g *gen) typeWalks(t ir.TypeRef) bool {
	switch {
	case t.Kind == types.Record, t.Kind == types.Variant:
		return g.needsWalk(t.Named)
	case t.Kind == types.List && t.Elem != nil:
		return g.typeWalks(*t.Elem)
	}
	return false
}
