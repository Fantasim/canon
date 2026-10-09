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

// checkGoDecoded is E8019 for what gen/go's data loaders and types-mode decoders cannot read in a class they decode (the plan's Decoded; CODEGEN.md §5.9, §5.13): a map (MapField), and in data mode an inline key folding onto another (InlineFoldedKey): a types-mode decoder checks no key, so none folds. A pairs field's element record of another package is read slot by slot into its hook's two parameters: check's E3316 leaves a pair record two scalar fields and no stored fn (WIRE.md §4.1), so nothing more is refused here.
func (s *stage) checkGoDecoded(u *unit, es *emitSite) {
	pl := PlanGoNames(s.view(u, es), es.e)
	shape := objectShape{extras: s.objectExtras(u, u.values)}
	for _, class := range packageClasses(u.p) {
		if !pl.Decoded(class) {
			continue
		}
		fields, fns := classBody(class)
		for _, site := range s.decodedSites(fields, readFns(es.e, fns)) {
			s.checkDecodedType(u, es, site)
		}
		if es.e.Mode == ModeData {
			s.checkInlineFolds(u, es, class, shape)
		}
	}
}

// checkGoDefaults is E8019 where a go types-mode decoder cannot write a field's constant default when its key is absent (CODEGEN.md §5.13), at each field of the package's classes (goTypesDefault).
func (s *stage) checkGoDefaults(u *unit, es *emitSite) { s.reportDefaults(u, es, goTypesDefault) }

// goTypesDefault is what gen/go's literal of field f's constant default cannot write: `DependentDefault` for a present value of a dependent type the decoder otherwise reads, whose branch only the discriminant it reads decides (CODEGEN.md §5.6), else a dependent value a record of the default holds, as a baked literal (literalDependent).
func goTypesDefault(e *Emit, fields []*Field, f *Field) (diag.Kind, bool) {
	switch {
	case f.Input != nil || !written(f.Default):
		return 0, false
	case typeHolds(&f.Type, isApp) && readsField(e, fields, f):
		return diag.KindDependentDefault, true
	}
	return literalDependent(&f.Type, f.Default)
}

// judgeGoDefaults is goTypesDefault at each field of c: a reader writes an absent key's default as the owner's own decoder would.
func judgeGoDefaults(_ *stage, _ *unit, e *Emit, c any) (diag.Kind, bool) {
	return firstDefault(e, c, goTypesDefault)
}

// checkResolvedLookups is E8019 `ResolvedLookupResult`: gen/go's data resolver has no walk for a lookup's cells holding a ref resolved at load (CODEGEN.md §5.8).
func (s *stage) checkResolvedLookups(u *unit, es *emitSite) {
	pl := PlanGoNames(s.view(u, es), es.e)
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
