package ir

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
)

// typeSite is one type an emit writes (a field, an export fn's parameter or result, a value, a constant) and where a finding on it points.
type typeSite struct {
	t                *TypeRef
	span             source.Span
	isConst, isValue bool
	isField          bool
}

// typeSites are every type an emit of u declares: its fields, its export fns' parameters and results, the values es selects, its constants.
func (s *stage) typeSites(u *unit, es *emitSite) []typeSite {
	var out []typeSite
	s.eachOwnField(u, func(_ string, f *Field) {
		out = append(out, typeSite{t: &f.Type, span: s.itemSpan(f, source.Span{}), isField: true})
	})
	for _, site := range s.ownFns(u) {
		for _, p := range site.fn.Params {
			out = append(out, typeSite{t: &p.Type, span: s.itemSpan(p, site.span())})
		}
		out = append(out, typeSite{t: &site.fn.Result, span: site.span()})
	}
	for _, v := range selectedValues(u, es.e) {
		out = append(out, typeSite{t: &v.v.Type, span: v.span().span(), isValue: true})
	}
	for _, c := range u.consts {
		out = append(out, typeSite{t: &c.c.Type, span: c.span(), isConst: true})
	}
	return out
}

// reportTypeSites is E8019 kind at every type site holding a type bad accepts; below is set for the sites whose own top type another rule judges.
func (s *stage) reportTypeSites(u *unit, es *emitSite, kind diag.Kind, bad func(*TypeRef) bool, below func(typeSite) bool) {
	for _, site := range s.typeSites(u, es) {
		held := typeHolds(site.t, bad)
		if below != nil && below(site) {
			held = typeHolds(site.t.Elem, bad) || typeHolds(site.t.Key, bad)
		}
		if held {
			u.reportGenConstruct(es, site.span, kind)
		}
	}
}

// typeHolds reports t, or its element, key or map value, that bad accepts; a named type's body is judged at its declaration.
func typeHolds(t *TypeRef, bad func(*TypeRef) bool) bool {
	if t == nil {
		return false
	}
	return bad(t) || typeHolds(t.Elem, bad) || typeHolds(t.Key, bad)
}

// checkOptionalElements is E8019 `OptionalElementList`: neither generator has a type for a list of optional elements (CODEGEN.md §4.2).
func (s *stage) checkOptionalElements(u *unit, es *emitSite) {
	s.reportTypeSites(u, es, diag.KindOptionalElementList, func(t *TypeRef) bool {
		return t.Kind == types.List && optionalElem(t)
	}, nil)
}

// checkOptionalMapValues is E8019 `OptionalMapValue`: neither generator has a type for a map of optional values (CODEGEN.md §4.2); a map a data loader reads is MapField alone.
func (s *stage) checkOptionalMapValues(u *unit, es *emitSite) {
	read := s.readSpans(u, es, isMap)
	for _, site := range s.typeSites(u, es) {
		if !read[site.span] && typeHolds(site.t, func(t *TypeRef) bool { return isMap(t) && optionalElem(t) }) {
			u.reportGenConstruct(es, site.span, diag.KindOptionalMapValue)
		}
	}
}

func optionalElem(t *TypeRef) bool { return t.Elem != nil && t.Elem.Kind == types.Optional }

// checkTableFields is E8019 `TableField`: gen/ts writes no table type but a table value's own container (CODEGEN.md §4.2, §5.9); gen/go and gen/cpp write a field of `table T` for a record of their own package, but not where a mode gives that record an id enum, whose members are the keys of a public table value, which a nested table's keys are not (§5.3).
func (s *stage) checkTableFields(u *unit, es *emitSite) {
	if es.e.Target != TargetGo && es.e.Target != TargetCpp {
		s.reportTypeSites(u, es, diag.KindTableField, func(t *TypeRef) bool { return t.Kind == types.Table },
			func(site typeSite) bool { return site.isValue })
		return
	}
	enumIDs := es.e.Mode != ModeData && es.e.Mode != ModeTypes
	tables := tableValueRecords(u.p)
	unwritten := func(t *TypeRef) bool {
		rec, ok := tableElem(*t)
		return t.Kind == types.Table && (!ok || rec.Pkg != u.p.Name || enumIDs && tables[rec])
	}
	anyTable := func(t *TypeRef) bool { return t.Kind == types.Table }
	for _, site := range s.typeSites(u, es) {
		bad := anyTable
		if site.isField {
			bad = unwritten
		}
		held := typeHolds(site.t, bad)
		if site.isValue {
			held = typeHolds(site.t.Elem, bad) || typeHolds(site.t.Key, bad)
		}
		if held {
			u.reportGenConstruct(es, site.span, diag.KindTableField)
		}
	}
}

// checkCaseFields is E8019 `CaseField` where a generator refuses a case used as a type (decision 219): gen/cpp stores none, gen/go has no type for one without fields and its data loader reads none.
func (s *stage) checkCaseFields(u *unit, es *emitSite) {
	cpp := es.e.Target == TargetCpp
	bad := func(t *TypeRef) bool {
		return t.Kind == types.Case && (cpp || t.Case != nil && len(t.Case.Fields) == 0)
	}
	read := s.readSpans(u, es, func(t *TypeRef) bool { return t.Kind == types.Case })
	for _, site := range s.typeSites(u, es) {
		switch {
		case cpp && site.isConst: // RecordConstant's
		case site.isConst && (typeHolds(site.t.Elem, bad) || typeHolds(site.t.Key, bad)), !site.isConst && (read[site.span] || typeHolds(site.t, bad)):
			u.reportGenConstruct(es, site.span, diag.KindCaseField)
		}
	}
}

// readSpans are the spans of what es's decoders read holding a type bad accepts: every class for gen/cpp, the plan's decoded ones for gen/go; none but where decodesClasses.
func (s *stage) readSpans(u *unit, es *emitSite, bad func(*TypeRef) bool) map[source.Span]bool {
	out := map[source.Span]bool{}
	if !decodesClasses(es.e) {
		return out
	}
	var pl *GoNamePlan
	if es.e.Target == TargetGo {
		pl = PlanGoNames(u.p, es.e)
	}
	for _, class := range packageClasses(u.p) {
		if pl != nil && !pl.Decoded(class) {
			continue
		}
		fields, fns := classBody(class)
		for _, site := range s.decodedSites(fields, readFns(es.e, fns)) {
			out[site.span] = out[site.span] || decodedHolds(site.t, bad)
		}
	}
	return out
}
