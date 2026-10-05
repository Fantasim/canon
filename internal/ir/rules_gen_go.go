package ir

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
)

// checkForeignTableLookups is E8019 `ForeignTableLookupParam`: a baked lookup's ref parameter is indexed by its table's id enum, which a table of another package has only when that package's emit for the target is baked or embedded (CODEGEN.md §5.10, DECISIONS 323).
func (s *stage) checkForeignTableLookups(u *unit, es *emitSite) {
	for _, site := range s.ownFns(u) {
		if site.fn.Kind != FnLookup {
			continue
		}
		for _, p := range site.fn.Params {
			if noIDEnum(u.p, es.e.Target, p.Type) {
				u.reportGenConstruct(es, s.itemSpan(p, site.span()), diag.KindForeignTableLookupParam)
			}
		}
	}
}

// noIDEnum reports a ref into a public table of another package than p whose emit of target t, as p imports it, gives the table no id enum: neither baked nor embedded, or no emit p imports (CODEGEN.md §5.3, §5.10).
func noIDEnum(p *Package, t Target, ref TypeRef) bool {
	r := ref.Ref
	if ref.Kind != types.Ref || r == nil || r.Coll != types.CollLet || r.Local || r.Keyed || r.Pkg == p.Name {
		return false
	}
	e := ownerEmit(p, r.Pkg, t)
	return e == nil || e.Mode != ModeBaked && e.Mode != ModeEmbedded
}

// checkGoDecoded is E8019 for what gen/go's data loaders cannot read in a class they decode (the plan's Decoded; CODEGEN.md §5.9): a map (MapField), an inline key folding onto another (InlineFoldedKey). A pairs field's element record of another package is read slot by slot into its hook's two parameters: check's E3316 leaves a pair record two scalar fields and no stored fn (WIRE.md §4.1), so nothing more is refused here.
func (s *stage) checkGoDecoded(u *unit, es *emitSite) {
	pl := PlanGoNames(u.p, es.e)
	shape := objectShape{extras: s.objectExtras(u, u.values)}
	for _, class := range packageClasses(u.p) {
		if !pl.Decoded(class) {
			continue
		}
		fields, fns := classBody(class)
		for _, site := range s.decodedSites(fields, fns) {
			s.checkDecodedType(u, es, site)
		}
		s.checkInlineFolds(u, es, class, shape)
	}
}

// checkResolvedLookups is E8019 `ResolvedLookupResult`: gen/go's data resolver has no walk for a lookup's cells holding a ref resolved at load (CODEGEN.md §5.8).
func (s *stage) checkResolvedLookups(u *unit, es *emitSite) {
	pl := PlanGoNames(u.p, es.e)
	for _, t := range u.p.Types {
		for _, class := range resolvedClasses(pl, t) {
			_, fns := classBody(class)
			s.reportResolvedLookups(u, es, pl, fns)
		}
	}
}

// reportResolvedLookups is `ResolvedLookupResult` at each lookup of fns whose cells a loader would walk.
func (s *stage) reportResolvedLookups(u *unit, es *emitSite, pl *GoNamePlan, fns []*ExportFn) {
	for _, fn := range fns {
		if fn.Kind == FnLookup && walksAtLoad(pl, &fn.Result) {
			u.reportGenConstruct(es, s.itemSpan(fn, source.Span{}), diag.KindResolvedLookupResult)
		}
	}
}

// resolvedClasses are the records and cases of t gen/go writes a resolver for: decoded, holding a ref resolved at load.
func resolvedClasses(pl *GoNamePlan, t Type) []any {
	switch x := t.(type) {
	case *Record:
		if pl.NeedsWalk(x) {
			return []any{x}
		}
	case *Variant:
		var out []any
		for _, c := range casesWithFields(x) {
			if pl.NeedsWalk(x) && pl.NeedsWalk(c) {
				out = append(out, c)
			}
		}
		return out
	}
	return nil
}

// walksAtLoad reports a result holding, through lists, a class whose refs a loader resolves.
func walksAtLoad(pl *GoNamePlan, t *TypeRef) bool {
	if t.Kind == types.Optional && t.Elem != nil {
		t = t.Elem
	}
	for ; t != nil; t = t.Elem {
		if t.Kind == types.Record || t.Kind == types.Variant {
			return pl.NeedsWalk(t.Named)
		}
		if t.Kind != types.List {
			return false
		}
	}
	return false
}
