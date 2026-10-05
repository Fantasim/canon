package ir

import (
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
)

// checkTSDecoded is E8019 where gen/ts's decoders cannot read what a class they decode holds (CODEGEN.md §5.6, §5.13, DECISIONS 278), at each field and stored fn (tsField, tsUnreadType). A data emit decodes the classes its values reach, a types emit every class; another package's classes are read with this package's own readers (§2.8, DECISIONS 323), judged at the site that reaches them (checkForeignReads).
func (s *stage) checkTSDecoded(u *unit, es *emitSite) {
	decoded := tsDecodedClasses(u, es.e)
	for _, class := range tsClasses(u.p) {
		if decoded[class] {
			s.checkTSClass(u, es, class)
		}
	}
}

// checkTSClass reports each field and stored fn of a decoded class that gen/ts cannot read, and each field default its reader cannot write.
func (s *stage) checkTSClass(u *unit, es *emitSite, class any) {
	fields, fns := classBody(class)
	for _, f := range fields {
		if kind, bad := tsField(fields, f); bad {
			u.reportGenConstruct(es, s.itemSpan(f, source.Span{}), kind)
		}
	}
	for _, fn := range readFns(es.e, fns) {
		if kind, bad := tsUnreadType(&fn.Result, true); fn.Kind != FnTranslated && bad {
			u.reportGenConstruct(es, s.itemSpan(fn, source.Span{}), kind)
		}
	}
}

// tsUnread is what gen/ts cannot read of field f of fields: a dependent value held, itself or through lists, is read with its discriminant when DiscFields finds it, of any package's dependent type (gen/ts reads as gen/go does; DECISIONS 320), else `DependentType`; a pairs slot's is refused (tsUnreadType).
func tsUnread(fields []*Field, f *Field) (diag.Kind, bool) {
	if f.Pairs != nil {
		for _, site := range readFieldSites(f, source.Span{}) {
			if kind, bad := tsUnreadType(site.t, true); bad {
				return kind, true
			}
		}
		return 0, false
	}
	app := HeldApp(f.Type)
	if app != nil && DiscFields(fields, *app) == nil {
		return diag.KindDependentType, true
	}
	return tsUnreadType(&f.Type, app == nil)
}

// tsUnreadType is what gen/ts cannot read in t, through its elements, keys and map values: `DependentOutsideField` for a dependent map or, when noDisc (no discriminant reaches it), a dependent value; `CaseField` for a case used as a type (no reader reads one, as in gen/go's loader).
func tsUnreadType(t *TypeRef, noDisc bool) (diag.Kind, bool) {
	switch {
	case typeHolds(t, func(t *TypeRef) bool { return t.Kind == types.DepMap || noDisc && isApp(t) }):
		return diag.KindDependentOutsideField, true
	case typeHolds(t, func(t *TypeRef) bool { return t.Kind == types.Case }):
		return diag.KindCaseField, true
	}
	return 0, false
}

// tsDecodedClasses are the classes gen/ts decodes: in types mode every one; in data mode those its selected values reach, through fields (a pairs field's pair record's fields), stored fn results, lists, optionals, maps and tables, a variant reaching its cases with an interface (CODEGEN.md §5.13; gen/ts need).
func tsDecodedClasses(u *unit, e *Emit) map[any]bool {
	out := map[any]bool{}
	if e.Mode == ModeTypes {
		for _, class := range tsClasses(u.p) {
			out[class] = true
		}
		return out
	}
	var reach func(class any)
	reach = func(class any) {
		if out[class] {
			return
		}
		out[class] = true
		for _, to := range tsHeldClasses(u.p.Name, class, e) {
			reach(to)
		}
	}
	for _, v := range selectedValues(u, e) {
		if root := goRootClass(v.v); root != nil && pkgOf(root.(Type)) == u.p.Name {
			reach(root)
		}
	}
	return out
}

// tsHeldClasses are the own classes a reader of class reads: a variant's cases with an interface, the records and variants a record's or case's read types hold anywhere.
func tsHeldClasses(own string, class any, e *Emit) []any {
	if v, ok := class.(*Variant); ok {
		return tsCases(v)
	}
	fields, fns := classBody(class)
	var sites []*TypeRef
	for _, f := range fields {
		if f.Input != nil {
			continue
		}
		for _, site := range readFieldSites(f, source.Span{}) {
			sites = append(sites, site.t)
		}
	}
	for _, fn := range readFns(e, fns) {
		if fn.Kind != FnTranslated {
			sites = append(sites, &fn.Result)
		}
	}
	var out []any
	for _, t := range sites {
		typeHolds(t, func(t *TypeRef) bool {
			if (t.Kind == types.Record || t.Kind == types.Variant) && pkgOf(t.Named) == own && !slices.Contains(out, any(t.Named)) {
				out = append(out, t.Named)
			}
			return false
		})
	}
	return out
}

// tsClasses are the classes gen/ts reads with a reader of their own, in declaration order: each record, each variant then its cases with fields or export fns.
func tsClasses(p *Package) []any {
	var out []any
	for _, t := range p.Types {
		switch x := t.(type) {
		case *Record:
			out = append(out, x)
		case *Variant:
			out = append(append(out, x), tsCases(x)...)
		}
	}
	return out
}

// tsCases are a variant's cases written as an interface with a reader: those with fields or export fns (gen/ts hasInterface).
func tsCases(v *Variant) []any {
	var out []any
	for _, c := range v.Cases {
		if len(c.Fields) > 0 || len(c.Methods) > 0 {
			out = append(out, c)
		}
	}
	return out
}

// tsField is what a ts reader cannot read or write of field f: tsUnread, then its default (tsDefault); an input or a Never? field is not read.
func tsField(fields []*Field, f *Field) (diag.Kind, bool) {
	if f.Input != nil || f.Optional && f.Type.Kind == types.Never {
		return 0, false
	}
	if kind, bad := tsUnread(fields, f); bad {
		return kind, true
	}
	return tsDefault(f)
}
