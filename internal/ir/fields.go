package ir

import (
	"cmp"
	"regexp"
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// fields are the IR fields of a record or case with their wire mapping (WIRE.md §4, §5); a field's type loses its outer `?`, which Optional keeps. params are the check objects of the owner record's own value parameters (nil for a case), so fieldDefault reads only those, not a lambda's own parameter.
func (s *stage) fields(fs []*types.Field, items []syntax.RecordItem, owner check.Object, params map[check.Object]bool) []*Field {
	out := make([]*Field, 0, len(fs))
	for _, f := range fs {
		t, opt := f.Type, false
		if o, ok := t.Base().(*types.OptionalType); ok {
			t, opt = o.Elem, true
		}
		fd := &Field{
			Name: f.Name, Doc: f.Doc, Type: s.ref(t), Optional: opt, NoneWire: f.NoneWire, Unit: f.Unit,
			Enc: f.Enc, Inline: f.Inline, Pairs: f.Pairs, Stable: f.Stable, Deprecated: f.Deprecated != nil, Input: f.Input,
		}
		if !f.Inline && f.Pairs == nil {
			fd.WirePath = f.WirePath
		}
		if f.Input != nil {
			fd.Range, fd.Patterns = ownRefinements(t)
		}
		n := nameOverrides(f.Annotations)
		fd.Go, fd.TS = n.goName, n.ts
		fd.Cpp = fieldCpp(f.Annotations)
		fd.BigInt = hasFlag(annotation(f.Annotations, syntax.AnnTS), syntax.ArgBigint)
		s.fieldDefault(fd, f, owner, params)
		out = append(out, fd)
		site := &fieldSite{tf: f}
		if d := fieldDecl(items, f.Name); d != nil {
			site.declSite = s.site(d.Name, d.Name)
		}
		s.fieldSites[fd] = site
	}
	return out
}

// recordParams are the check objects of r's own declared value parameters, or nil for one without any; a field default that reads one of these is Computed (decision 80), but a lambda bound with the same identifier kind inside that default is not.
func (s *stage) recordParams(r *types.RecordType) map[check.Object]bool {
	if r.Decl == nil || len(r.Decl.Params) == 0 {
		return nil
	}
	out := map[check.Object]bool{}
	for _, p := range r.Decl.Params {
		if o := s.info.Defs[p.Name]; o != nil {
			out[o] = true
		}
	}
	return out
}

// fieldDecl is the declaration of the field named name among a body's items.
func fieldDecl(items []syntax.RecordItem, name string) *syntax.FieldDecl {
	for _, it := range items {
		if d, ok := it.(*syntax.FieldDecl); ok && d.Name != nil && d.Name.Name == name {
			return d
		}
	}
	return nil
}

// fieldDefault is a constant default, folded, or Computed for one reading earlier fields or the record's parameters (TYPES.md §15, decision 80); a broken owner's default is never folded (decisions 209, 213).
func (s *stage) fieldDefault(fd *Field, f *types.Field, owner check.Object, params map[check.Object]bool) {
	if f.Default == nil {
		return
	}
	if s.readsInstance(f.Default, params) {
		fd.Computed = true
		return
	}
	if s.in.Fold == nil || owner == nil || s.info.Broken[owner] {
		return
	}
	if v, ok := s.in.Fold.Fold(s.ctx, owner, f.Default, s.info); ok {
		fd.Default = v
	}
}

// readsInstance reports an expression naming a field, or one of params (the owner record's own
// value parameters); a lambda's own parameter, of the same check.ObjParam kind but not in
// params, does not count.
func (s *stage) readsInstance(e syntax.Expr, params map[check.Object]bool) bool {
	found := false
	syntax.Inspect(e, func(n syntax.Node) bool {
		if id, ok := n.(*syntax.IdentExpr); ok {
			if o := s.info.Uses[id]; o != nil && (o.Kind() == check.ObjField || params[o]) {
				found = true
			}
		}
		return !found
	})
	return found
}

// ownRefinements are the range and patterns written on a type, through aliases (CODEGEN.md §5.12): nested ranges as their intersection, and every pattern (TYPES.md §7.4: all are checked) innermost first in alias-chain order (not declaration order), as eval checks them at storage points (EVALUATION.md §4.3); the order is not observable in a loader, whose every pattern failure reads the same line. A pattern repeated along the chain is kept once: one table, one regexp.
func ownRefinements(t types.Type) (*types.Bound, []*regexp.Regexp) {
	var rng *types.Bound
	var pats []*regexp.Regexp
	float := t.Base().Kind() == types.Float
	for t != nil {
		switch x := t.(type) {
		case *types.Refined:
			rng = intersect(rng, x.Range, float)
			if x.Pattern != nil {
				pats = append(pats, x.Pattern)
			}
			t = x.Of
		case *types.Alias:
			t = x.Def
		default:
			t = nil
		}
	}
	slices.Reverse(pats)
	return rng, distinctPatterns(pats)
}

// distinctPatterns keeps the first of each pattern text, in order.
func distinctPatterns(pats []*regexp.Regexp) []*regexp.Regexp {
	var out []*regexp.Regexp
	for _, p := range pats {
		if !slices.ContainsFunc(out, func(q *regexp.Regexp) bool { return q.String() == p.String() }) {
			out = append(out, p)
		}
	}
	return out
}

// intersect is the values both bounds admit, nil when neither bounds; F compares for a Float base, I otherwise.
func intersect(a, b *types.Bound, float bool) *types.Bound {
	if a == nil || b == nil {
		return cmp.Or(a, b)
	}
	less := func(x, y types.Limit) bool { return x.I < y.I }
	if float {
		less = func(x, y types.Limit) bool { return x.F < y.F }
	}
	out := *a
	if b.HasLo && (!out.HasLo || less(out.Lo, b.Lo)) {
		out.Lo, out.HasLo = b.Lo, true
	}
	switch {
	case !b.HasHi:
	case !out.HasHi || less(b.Hi, out.Hi):
		out.Hi, out.HasHi, out.HiIncluded = b.Hi, true, b.HiIncluded
	case !less(out.Hi, b.Hi):
		out.HiIncluded = out.HiIncluded && b.HiIncluded // equal ends: the exclusive one is tighter
	}
	return &out
}
