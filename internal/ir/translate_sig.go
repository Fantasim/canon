package ir

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// checkTranslatedParams is E9006 on a parameter of another kind and E9007 on a refinement not checkable at run time (CONFORMANCE.md §2.1); an optional one is E9003's; false when one is refused.
func (s *stage) checkTranslatedParams(u *unit, site *fnSite) bool {
	ok := true
	file := site.obj.File()
	for i, p := range site.sig.Params {
		if i >= len(site.decl.Params) || i >= len(site.fn.Params) {
			return false
		}
		dp, name := site.decl.Params[i], site.fn.Params[i].Name
		ty := p
		if o, isOpt := p.Base().(*types.OptionalType); isOpt {
			ty, ok = o.Elem, false
		}
		at := file.Span(dp.Name)
		if !scalarKinds[ty.Base().Kind()] || isAsset(ty) {
			u.report(diag.E9006.At(at, name, site.label, p))
			ok = false
			continue
		}
		if r := uncheckable(ty); r != nil {
			u.report(diag.E9007.AtParam(at, s.refinementSpan(file, dp.Type, r), name, site.label))
			ok = false
		}
	}
	return ok
}

// checkTranslatedResult is E9004 and E9007 on a translated fn's result: a scalar or a ref, not optional.
func (s *stage) checkTranslatedResult(u *unit, site *fnSite) bool {
	file, res := site.obj.File(), site.sig.Result
	var at source.Span
	if site.decl.Result != nil {
		at = file.Span(site.decl.Result)
	} else {
		at = site.span()
	}
	if !resultKinds[res.Base().Kind()] || isAsset(res) {
		u.report(diag.E9004.At(at, site.label, res))
		return false
	}
	if r := uncheckable(res); r != nil {
		u.report(diag.E9007.AtResult(at, s.refinementSpan(file, site.decl.Result, r), site.label))
		return false
	}
	return true
}

// isAsset reports an asset type, a String refined by asset(…) (TYPES.md §13.4).
func isAsset(t types.Type) bool {
	return refinement(t, func(r *types.Refined) bool { return r.Asset != nil }) != nil
}

// uncheckable is the first refinement generated code cannot check: `where`, a regex, a length.
func uncheckable(t types.Type) *types.Refined {
	return refinement(t, func(r *types.Refined) bool {
		return r.Where != nil || r.Pattern != nil || (r.Range != nil && r.Of.Base().Kind() != types.Int &&
			r.Of.Base().Kind() != types.Float && r.Of.Base().Kind() != types.Duration)
	})
}

// refinement is the first refinement of t, through aliases, that bad holds for.
func refinement(t types.Type, bad func(*types.Refined) bool) *types.Refined {
	for t != nil {
		switch x := t.(type) {
		case *types.Refined:
			if bad(x) {
				return x
			}
			t = x.Of
		case *types.Alias:
			t = x.Def
		default:
			return nil
		}
	}
	return nil
}

// refinementSpan is where the written type states r: its `where` predicate or its argument
// list; the whole written type when r comes from an alias it names.
func (s *stage) refinementSpan(file *syntax.File, written syntax.Type, r *types.Refined) source.Span {
	if written == nil {
		return file.Span(written)
	}
	var found syntax.Node
	syntax.Inspect(written, func(n syntax.Node) bool {
		if st, ok := n.(syntax.Type); ok && found == nil && s.info.TypeExprs[st] == types.Type(r) {
			found = n
		}
		return found == nil
	})
	switch x := found.(type) {
	case *syntax.WhereType:
		return file.Span(x.Pred)
	case *syntax.NamedType:
		if x.Args != nil {
			return file.Span(x.Args)
		}
	}
	return file.Span(written)
}
