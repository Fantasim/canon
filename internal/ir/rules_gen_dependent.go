package ir

import (
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// checkNeverDependents is E8019 `NeverDependent` at a dependent type every arm of which is Never, used or not: no generator writes one (log-2026-09-28 "Start").
func (s *stage) checkNeverDependents(u *unit, es *emitSite) {
	for _, t := range u.p.Types {
		if d, ok := t.(*Dependent); ok && len(d.Branches) == 0 {
			u.reportGenConstruct(es, s.decls[d].span(), diag.KindNeverDependent)
		}
	}
}

// checkGoDependentLiterals is E8019 where baked Go or C++ cannot write a dependent value (both read its discriminant down DiscFields, of any package's dependent type, DECISIONS 320): a selected value holding one, and a stored fn's results once, at the fn (CODEGEN.md §5.6; gen/go assignDependent); the kind is literalDependent's.
func (s *stage) checkGoDependentLiterals(u *unit, es *emitSite) {
	for _, v := range selectedValues(u, es.e) {
		if kind, bad := literalDependent(&v.v.Type, v.v.V); bad {
			u.reportGenConstruct(es, v.span().span(), kind)
		}
	}
	for _, site := range s.ownFns(u) {
		if site.fn.Kind == FnTranslated {
			continue
		}
		for _, r := range storedResults(site.fn) {
			if kind, bad := literalDependent(&site.fn.Result, r); bad {
				u.reportGenConstruct(es, site.span(), kind)
				break
			}
		}
	}
}

// literalDependent is what a literal of v, of type t, cannot write of a present dependent value: `DependentOutsideField` for one outside a record field (a value's or a result's own, a map's, a non-string value of a literal union over one; a string is written as one), `DependentType` for one of a field whose record does not hold its discriminant as DiscFields reads it (decided through a ref, an optional or a record parameter).
func literalDependent(t *TypeRef, v value.Value) (diag.Kind, bool) {
	if decodedHolds(t, isApp) && written(v) {
		return diag.KindDependentOutsideField, true
	}
	outside := literalHolds(t, v, func(t *TypeRef, v value.Value) bool {
		switch x := v.(type) {
		case *value.Map:
			return len(x.Keys) > 0 && (typeHolds(t.Key, isApp) || typeHolds(t.Elem, isApp))
		case *value.Record, *value.Str, *value.None:
			return false
		}
		return t.Kind == types.LitUnion && t.Elem != nil && t.Elem.Kind == types.TypeApp
	})
	if outside {
		return diag.KindDependentOutsideField, true
	}
	undecided := literalHolds(t, v, func(t *TypeRef, v value.Value) bool {
		r, ok := v.(*value.Record)
		return ok && unwrittenFields(t, r)
	})
	return diag.KindDependentType, undecided
}

// unwrittenFields reports a present value of a dependent field of the record value r, of type t, whose discriminant DiscFields does not read from the record's fields.
func unwrittenFields(t *TypeRef, r *value.Record) bool {
	if !recordKinds[t.Kind] || t.Named == nil {
		return false
	}
	decl, fields := ownFields(r.T), bodyFields(t, r)
	for _, f := range fields {
		app := HeldApp(f.Type)
		i := slices.IndexFunc(decl, func(d *types.Field) bool { return d.Name == f.Name })
		if app == nil || i < 0 || i >= len(r.Fields) || !written(r.Fields[i]) {
			continue
		}
		if DiscFields(fields, *app) == nil {
			return true
		}
	}
	return false
}

// written reports a value a literal writes: not none, and a list with an element that is written.
func written(v value.Value) bool {
	switch x := v.(type) {
	case nil, *value.None:
		return false
	case *value.List:
		return slices.ContainsFunc(x.Elems, written)
	}
	return true
}

func isApp(t *TypeRef) bool { return t.Kind == types.TypeApp }

// HeldApp is the type application t holds, itself or through lists, or nil: a field's elements share its discriminant (CODEGEN.md §5.6).
func HeldApp(t TypeRef) *TypeRef {
	app := &t
	for app.Kind == types.List && app.Elem != nil {
		app = app.Elem
	}
	if app.Kind != types.TypeApp {
		return nil
	}
	return app
}

// checkGoDecodedDependents is E8019 where gen/go's data loader cannot read a dependent value of a class it decodes (the plan's Decoded; CODEGEN.md §5.6; log-2026-09-25 "gen/go review round 1 calls").
func (s *stage) checkGoDecodedDependents(u *unit, es *emitSite) {
	pl := PlanGoNames(u.p, es.e)
	for _, class := range packageClasses(u.p) {
		if pl.Decoded(class) {
			s.reportDecodedDependents(u, es, class)
		}
	}
}

// checkCppDependents is E8019 where gen/cpp's loader, which decodes every class, cannot read a dependent value (CODEGEN.md §5.6).
func (s *stage) checkCppDependents(u *unit, es *emitSite) {
	for _, class := range packageClasses(u.p) {
		s.reportDecodedDependents(u, es, class)
	}
}

// reportDecodedDependents is decodedDependent at each field and stored fn of class whose dependent value the loader cannot read; a map is MapField's.
func (s *stage) reportDecodedDependents(u *unit, es *emitSite, class any) {
	fields, fns := classBody(class)
	for _, f := range fields {
		if kind, bad := fieldDependent(es.e, fields, f); bad {
			u.reportGenConstruct(es, s.itemSpan(f, source.Span{}), kind)
		}
	}
	for _, fn := range readFns(es.e, fns) {
		if fn.Kind != FnTranslated && unreadDependent(es.e, &fn.Result) {
			u.reportGenConstruct(es, s.itemSpan(fn, source.Span{}), diag.KindDependentOutsideField)
		}
	}
}

// fieldDependent is what a loader cannot read of a dependent value of field f of fields: `DependentType` for one it holds, itself or through lists, whose discriminant DiscFields does not read (a ref, an optional, a record parameter), `DependentOutsideField` for one elsewhere in it (a map, a literal union, a pairs slot); a map is MapField's where the mode reads none.
func fieldDependent(e *Emit, fields []*Field, f *Field) (diag.Kind, bool) {
	if f.Input != nil || f.Optional && f.Type.Kind == types.Never || readsField(e, fields, f) {
		return 0, false
	}
	if f.Pairs == nil && HeldApp(f.Type) != nil {
		return diag.KindDependentType, true
	}
	return diag.KindDependentOutsideField, true
}

// readsField reports that the loader reads field f of fields: no dependent type in it, or one application it holds, itself or through lists, whose discriminant DiscFields reads, of any package's dependent type (DECISIONS 320); a pairs field's slots are read without one.
func readsField(e *Emit, fields []*Field, f *Field) bool {
	if f.Pairs != nil {
		return !slices.ContainsFunc(readFieldSites(f, source.Span{}), func(site typeSite) bool { return unreadDependent(e, site.t) })
	}
	if app := HeldApp(f.Type); app != nil {
		return DiscFields(fields, *app) != nil
	}
	return !unreadDependent(e, &f.Type)
}

// unreadDependent reports a dependent type in t, a literal union's base included, or a dependent map: a map's key or value is the loader's limit (CODEGEN.md §5.9, DECISIONS 312), except where the mode's decoder refuses every map (MapField's).
func unreadDependent(e *Emit, t *TypeRef) bool {
	return (readsMaps(e) || !decodedHolds(t, isMap)) && typeHolds(t, func(x *TypeRef) bool { return isApp(x) || x.Kind == types.DepMap })
}

// DiscFields are the fields a loader, or a baked literal, reads from fields to app's discriminant, in order: the argument's then the match's wire path, through earlier fields of records held by value, none optional (check's E3806) or a ref (a WIRE.md §5.9 load-time resolution CODEGEN.md does not write), ending at a field of the discriminant's type; nil for any other argument, a record parameter's (§5.7) or a dependent map binder's (§4.2) included (TYPES.md §11.1, §11.2).
func DiscFields(fields []*Field, app TypeRef) []*Field {
	d, ok := app.Named.(*Dependent)
	if !ok || d.Disc == nil || d.DiscParam < 0 || d.DiscParam >= len(app.Args) || app.Args[d.DiscParam] == nil {
		return nil
	}
	src := app.Args[d.DiscParam]
	if src.From != types.ArgField {
		return nil
	}
	segs := slices.Concat(src.WirePath, d.DiscPath)
	var out []*Field
	for len(segs) > 0 {
		f := fieldAtWire(fields, segs)
		if f == nil || f.Optional || f.Type.Kind == types.Ref {
			return nil
		}
		segs, out = segs[len(f.WirePath):], append(out, f)
		fields = nil
		if rec, ok := f.Type.Named.(*Record); ok && f.Type.Kind == types.Record {
			fields = rec.Fields
		}
	}
	if len(out) == 0 {
		return nil
	}
	if last := out[len(out)-1]; last.Type.Kind != d.Disc.Kind || last.Type.Named != d.Disc.Named {
		return nil
	}
	return out
}

// fieldAtWire is the field of fields whose wire path starts segs, or nil.
func fieldAtWire(fields []*Field, segs []string) *Field {
	for _, f := range fields {
		if n := len(f.WirePath); n > 0 && n <= len(segs) && slices.Equal(f.WirePath, segs[:n]) {
			return f
		}
	}
	return nil
}
