package ir

import (
	"regexp"

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
			fd.Range, fd.Pattern = ownRefinements(t)
		}
		n := nameOverrides(f.Annotations)
		fd.Go, fd.TS = n.goName, n.ts
		fd.Cpp = fieldCpp(f.Annotations)
		fd.BigInt = hasFlag(annotation(f.Annotations, syntax.AnnTS), flagBigInt)
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

// fieldDefault is a constant default, folded, or Computed for one reading earlier fields or the record's parameters (TYPES.md §15, decision 80).
func (s *stage) fieldDefault(fd *Field, f *types.Field, owner check.Object, params map[check.Object]bool) {
	if f.Default == nil {
		return
	}
	if s.readsInstance(f.Default, params) {
		fd.Computed = true
		return
	}
	if s.in.Fold == nil || owner == nil {
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

// ownRefinements are the range and pattern written on an input field's type, through aliases (CODEGEN.md §5.12: checked at run time).
func ownRefinements(t types.Type) (*types.Bound, *regexp.Regexp) {
	var rng *types.Bound
	var pat *regexp.Regexp
	for t != nil {
		switch x := t.(type) {
		case *types.Refined:
			if rng == nil {
				rng = x.Range
			}
			if pat == nil {
				pat = x.Pattern
			}
			t = x.Of
		case *types.Alias:
			t = x.Def
		default:
			return rng, pat
		}
	}
	return rng, pat
}
