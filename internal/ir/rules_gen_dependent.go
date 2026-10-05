package ir

import (
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// checkNeverDependents is E8019 `DependentType` at a dependent type every arm of which is Never, used or not: no generator writes one (log-2026-09-28 "Start").
func (s *stage) checkNeverDependents(u *unit, es *emitSite) {
	for _, t := range u.p.Types {
		if d, ok := t.(*Dependent); ok && len(d.Branches) == 0 {
			u.reportGenConstruct(es, s.decls[d].span(), diag.KindDependentType)
		}
	}
}

// checkGoDependentLiterals is E8019 `DependentType` where baked Go or C++ cannot write a dependent value (both read its discriminant down DiscFields, of an own type): a selected value holding one, and a stored fn's results once, at the fn (CODEGEN.md §5.6; gen/go assignDependent).
func (s *stage) checkGoDependentLiterals(u *unit, es *emitSite) {
	own := u.p.Name
	for _, v := range selectedValues(u, es.e) {
		if bakedDependent(own, &v.v.Type, v.v.V) {
			u.reportGenConstruct(es, v.span().span(), diag.KindDependentType)
		}
	}
	for _, site := range s.ownFns(u) {
		fn := site.fn
		results := func(r value.Value) bool { return bakedDependent(own, &fn.Result, r) }
		if fn.Kind != FnTranslated && slices.ContainsFunc(storedResults(fn), results) {
			u.reportGenConstruct(es, site.span(), diag.KindDependentType)
		}
	}
}

// bakedDependent reports a present dependent value of v, of type t, that baked gen/go does not write: one outside a field (a value's or a result's own, a map's), one of a field whose record does not hold its discriminant as goDiscRead reads it, or a non-string value of a literal union over one (a string is written as one).
func bakedDependent(own string, t *TypeRef, v value.Value) bool {
	return literalDependent(own, t, v, goDiscRead)
}

// literalDependent is bakedDependent for a generator whose literals read a discriminant as reads does.
func literalDependent(own string, t *TypeRef, v value.Value, reads discReader) bool {
	if decodedHolds(t, isApp) && written(v) {
		return true
	}
	return literalHolds(t, v, func(t *TypeRef, v value.Value) bool {
		switch x := v.(type) {
		case *value.Record:
			return unwrittenFields(own, t, x, reads)
		case *value.Map:
			return len(x.Keys) > 0 && (typeHolds(t.Key, isApp) || typeHolds(t.Elem, isApp))
		case *value.Str, *value.None:
			return false
		}
		return t.Kind == types.LitUnion && t.Elem != nil && t.Elem.Kind == types.TypeApp
	})
}

// unwrittenFields reports a present value of a dependent field of the record value r, of type t, whose discriminant reads does not read from the record's fields.
func unwrittenFields(own string, t *TypeRef, r *value.Record, reads discReader) bool {
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
		if !reads(own, fields, app) {
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

// checkGoDecodedDependents is E8019 `DependentType` where gen/go's data loader cannot read a dependent value of a class it decodes (the plan's Decoded; CODEGEN.md §5.6; log-2026-09-25 "gen/go review round 1 calls").
func (s *stage) checkGoDecodedDependents(u *unit, es *emitSite) {
	pl := PlanGoNames(u.p, es.e)
	for _, class := range packageClasses(u.p) {
		if pl.Decoded(class) {
			s.reportDecodedDependents(u, es, class, goDiscRead)
		}
	}
}

// checkCppDependents is E8019 `DependentType` where gen/cpp's loader, which decodes every class, cannot read a dependent value (CODEGEN.md §5.6).
func (s *stage) checkCppDependents(u *unit, es *emitSite) {
	for _, class := range packageClasses(u.p) {
		s.reportDecodedDependents(u, es, class, cppDiscRead)
	}
}

// discReader reports that a loader reads app's discriminant from fields, those of a class of package own.
type discReader func(own string, fields []*Field, app *TypeRef) bool

// reportDecodedDependents is `DependentType` at each field and stored fn of class whose dependent value the loader cannot read; a map is MapField's.
func (s *stage) reportDecodedDependents(u *unit, es *emitSite, class any, reads discReader) {
	fields, fns := classBody(class)
	for _, f := range fields {
		if f.Input == nil && (!f.Optional || f.Type.Kind != types.Never) && !readsField(u.p.Name, es.e, fields, f, reads) {
			u.reportGenConstruct(es, s.itemSpan(f, source.Span{}), diag.KindDependentType)
		}
	}
	for _, fn := range readFns(es.e, fns) {
		if fn.Kind != FnTranslated && unreadDependent(es.e, &fn.Result) {
			u.reportGenConstruct(es, s.itemSpan(fn, source.Span{}), diag.KindDependentType)
		}
	}
}

// readsField reports that the loader reads field f of fields: no dependent type in it, or one application it holds, itself or through lists, whose discriminant it reads; a pairs field's slots are read without one.
func readsField(own string, e *Emit, fields []*Field, f *Field, reads discReader) bool {
	if f.Pairs != nil {
		return !slices.ContainsFunc(readFieldSites(f, source.Span{}), func(site typeSite) bool { return unreadDependent(e, site.t) })
	}
	if app := HeldApp(f.Type); app != nil {
		return reads(own, fields, app)
	}
	return !unreadDependent(e, &f.Type)
}

// unreadDependent reports a dependent type in t, a literal union's base included, or a dependent map: a map's key or value is DependentType's (CODEGEN.md §5.9, DECISIONS 312), except where the mode's decoder refuses every map (MapField's).
func unreadDependent(e *Emit, t *TypeRef) bool {
	return (e.Mode == ModeData || !decodedHolds(t, isMap)) && typeHolds(t, func(x *TypeRef) bool { return isApp(x) || x.Kind == types.DepMap })
}

// goDiscRead reports that gen/go reads app's discriminant: DiscFields finds it, of an own dependent type (its decoder is unexported, so another package's is refused).
func goDiscRead(own string, fields []*Field, app *TypeRef) bool {
	d, ok := app.Named.(*Dependent)
	return ok && d.Pkg == own && DiscFields(fields, *app) != nil
}

// cppDiscRead reports that gen/cpp reads app's discriminant: DiscFields finds it, of any package's dependent type (it calls that package's Decode<Alias>).
func cppDiscRead(_ string, fields []*Field, app *TypeRef) bool {
	return DiscFields(fields, *app) != nil
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
