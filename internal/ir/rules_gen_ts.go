package ir

import (
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
)

// checkTSDecoded is E8019 where gen/ts's decoders cannot read what a class they decode holds (CODEGEN.md §5.6, §5.13, DECISIONS 278): `DependentType` for a dependent map, a dependent value whose discriminant the reader does not hold (in a map, a literal union, a pairs slot or a fn result, or read through a ref or a record parameter), or a dependent type of another package; `ForeignDataRecord` for a record or variant of a package whose ts emit is not in types mode, the one that has public decoders; `CaseField` for a case used as a type; `DependentType` or `RecordCycleThroughMethod` for a field default its reader cannot write (tsDefault). A data emit decodes the classes its values reach, a types emit every class; a data value whose rows or record are another package's decodes through that package's types-mode decoder, so it is `ForeignDataRecord` when there is none, or when that record is a row there (its decoder requires `$id`).
func (s *stage) checkTSDecoded(u *unit, es *emitSite) {
	decoded, cycles, strict := tsDecodedClasses(u, es.e), newLiteralCycles(s), s.tsStrictRows(u)
	for _, class := range tsClasses(u.p) {
		if decoded[class] {
			s.checkTSClass(u, es, class, cycles, strict)
		}
	}
	for _, v := range selectedValues(u, es.e) {
		r := goRootClass(v.v)
		if r == nil {
			continue
		}
		root := &TypeRef{Kind: types.Record, Named: r.(Type)}
		if tsForeignClass(u, root) || tsPlainRow(root, strict) && v.v.Type.Kind != types.Table {
			u.reportGenConstruct(es, v.span().span(), diag.KindForeignDataRecord)
		}
	}
}

// checkTSClass reports each field and stored fn of a decoded class that gen/ts cannot read, and each field default its reader cannot write.
func (s *stage) checkTSClass(u *unit, es *emitSite, class any, cycles *literalCycles, strict func(*Record) bool) {
	fields, fns := classBody(class)
	for _, f := range fields {
		if f.Input != nil || f.Optional && f.Type.Kind == types.Never {
			continue
		}
		if kind, bad := tsField(u, fields, f, cycles, strict); bad {
			u.reportGenConstruct(es, s.itemSpan(f, source.Span{}), kind)
		}
	}
	for _, fn := range readFns(es.e, fns) {
		if kind, bad := tsUnreadType(u, &fn.Result, true); fn.Kind != FnTranslated && bad {
			u.reportGenConstruct(es, s.itemSpan(fn, source.Span{}), kind)
		}
	}
}

// tsUnread is what gen/ts cannot read of field f of fields: a dependent value held, itself or through lists, is read with its discriminant when goDiscRead finds it (DiscFields, of an own dependent type: gen/ts reads as gen/go does); any other is refused, as is a pairs slot's.
func tsUnread(u *unit, fields []*Field, f *Field) (diag.Kind, bool) {
	if f.Pairs != nil {
		for _, site := range readFieldSites(f, source.Span{}) {
			if kind, bad := tsUnreadType(u, site.t, true); bad {
				return kind, true
			}
		}
		return 0, false
	}
	app := HeldApp(f.Type)
	if app != nil && !goDiscRead(u.p.Name, fields, app) {
		return diag.KindDependentType, true
	}
	return tsUnreadType(u, &f.Type, app == nil)
}

// tsUnreadType is what gen/ts cannot read in t, through its elements, keys and map values: a dependent map, a dependent value when noDisc (no discriminant reaches it), another package's dependent type, another package's record or variant without a types-mode emit, or a case used as a type (no reader reads one, as in gen/go's loader).
func tsUnreadType(u *unit, t *TypeRef, noDisc bool) (diag.Kind, bool) {
	own := u.p.Name
	switch {
	case typeHolds(t, func(t *TypeRef) bool { return t.Kind == types.DepMap }):
		return diag.KindDependentType, true
	case typeHolds(t, func(t *TypeRef) bool { return isApp(t) && (noDisc || pkgOf(t.Named) != own) }):
		return diag.KindDependentType, true
	case typeHolds(t, func(t *TypeRef) bool { return tsForeignClass(u, t) }):
		return diag.KindForeignDataRecord, true
	case typeHolds(t, func(t *TypeRef) bool { return t.Kind == types.Case }):
		return diag.KindCaseField, true
	}
	return 0, false
}

// tsForeignClass reports a record or variant of another package whose ts emit is not in types mode (a table of one is checkTSForeignTables').
func tsForeignClass(u *unit, t *TypeRef) bool {
	if t.Kind != types.Record && t.Kind != types.Variant || pkgOf(t.Named) == u.p.Name {
		return false
	}
	e := tsEmitOf(u.p, pkgOf(t.Named))
	return e != nil && e.Mode != ModeTypes
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

// tsPlainRow reports t a record of another package whose own reader requires `$id` (strict), held in a plain position: its public decoder would refuse every value.
func tsPlainRow(t *TypeRef, strict func(*Record) bool) bool {
	r, ok := t.Named.(*Record)
	return ok && t.Kind == types.Record && strict(r)
}

// tsField is what a ts reader cannot read or write of field f: tsUnread, a plain foreign row (tsPlainRow), then its default (tsDefault).
func tsField(u *unit, fields []*Field, f *Field, cycles *literalCycles, strict func(*Record) bool) (diag.Kind, bool) {
	if kind, bad := tsUnread(u, fields, f); bad {
		return kind, true
	}
	if typeHolds(&f.Type, func(t *TypeRef) bool { return tsPlainRow(t, strict) }) {
		return diag.KindForeignDataRecord, true
	}
	return tsDefault(f, cycles)
}
