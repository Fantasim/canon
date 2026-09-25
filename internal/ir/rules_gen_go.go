package ir

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
)

// checkForeignTables is E8019 `CrossPackageBakedValue`: gen/go indexes every table value, selected or not, by its own record (CODEGEN.md §5.3).
func (s *stage) checkForeignTables(u *unit, es *emitSite) {
	for _, v := range u.values {
		if foreignTable(u.p.Name, v.v.Type) {
			u.reportGenConstruct(es, v.span().span(), diag.KindCrossPackageBakedValue)
		}
	}
}

// foreignTable reports a table whose element is not a record of package own.
func foreignTable(own string, t TypeRef) bool {
	if t.Kind != types.Table || t.Elem == nil {
		return false
	}
	rec, ok := t.Elem.Named.(*Record)
	return !ok || rec.Pkg != own
}

// checkForeignTableLookups is E8019 `ForeignTableLookupParam`: baked Go enumerates a lookup's ref domain from its own package's tables only (CODEGEN.md §5.10).
func (s *stage) checkForeignTableLookups(u *unit, es *emitSite) {
	for _, site := range s.ownFns(u) {
		if site.fn.Kind != FnLookup {
			continue
		}
		for _, p := range site.fn.Params {
			if r := p.Type.Ref; p.Type.Kind == types.Ref && r != nil && r.Coll == types.CollLet && !r.Local && !r.Keyed && r.Pkg != u.p.Name {
				u.reportGenConstruct(es, s.itemSpan(p, site.span()), diag.KindForeignTableLookupParam)
			}
		}
	}
}

// checkGoDecoded is E8019 for what gen/go's data loaders cannot read in a class they decode (the plan's Decoded; CODEGEN.md §5.9) and in a value's root.
func (s *stage) checkGoDecoded(u *unit, es *emitSite) {
	pl := PlanGoNames(u.p, es.e)
	shape := objectShape{extras: s.objectExtras(u, u.values)}
	own := u.p.Name
	for _, class := range packageClasses(u.p) {
		if !pl.Decoded(class) {
			continue
		}
		fields, fns := classBody(class)
		for _, site := range s.decodedSites(fields, fns) {
			s.checkDecodedType(u, es, site, true)
			if decodedHolds(site.t, func(t *TypeRef) bool { return foreignClass(own, t) }) {
				u.reportGenConstruct(es, site.span, diag.KindForeignDataRecord)
			}
		}
		s.checkInlineFolds(u, es, class, shape)
	}
	for _, v := range selectedValues(u, es.e) {
		if root, ok := goRootClass(v.v).(Type); ok && !foreignTable(own, v.v.Type) && pkgOf(root) != own {
			u.reportGenConstruct(es, v.span().span(), diag.KindForeignDataRecord)
		}
	}
}

// foreignClass reports a record or variant of a package other than own, which a data loader decodes with that package's unexported decoder.
func foreignClass(own string, t *TypeRef) bool {
	return (t.Kind == types.Record || t.Kind == types.Variant) && t.Named != nil && pkgOf(t.Named) != own
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
