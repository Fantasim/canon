package ir

import (
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// dependentReader is how a target's data loader reads a dependent field: through lists or only as the field's own type, and whether it can read the discriminant of an application.
type dependentReader struct {
	throughLists bool
	disc         func(own string, fields []*Field, app *TypeRef) bool
}

var (
	goDependents  = dependentReader{disc: goDiscRead}
	cppDependents = dependentReader{throughLists: true, disc: cppDiscRead}
)

// checkGoDependentLiterals is E8019 `DependentType` where baked Go would write a dependent value as a literal: a selected value holding one, and a stored fn's results once, at the fn (CODEGEN.md §5.6; gen/go's dependentLiteralFormat).
func (s *stage) checkGoDependentLiterals(u *unit, es *emitSite) {
	for _, v := range selectedValues(u, es.e) {
		if literalHolds(&v.v.Type, v.v.V, dependentLiteral) {
			u.reportGenConstruct(es, v.span().span(), diag.KindDependentType)
		}
	}
	for _, site := range s.ownFns(u) {
		fn := site.fn
		results := func(r value.Value) bool { return literalHolds(&fn.Result, r, dependentLiteral) }
		if fn.Kind != FnTranslated && slices.ContainsFunc(storedResults(fn), results) {
			u.reportGenConstruct(es, site.span(), diag.KindDependentType)
		}
	}
}

// dependentLiteral is a present dependent value, or a non-string value of a literal union over a dependent type (a string is written as one): gen/go writes neither as a literal.
func dependentLiteral(t *TypeRef, v value.Value) bool {
	if _, none := v.(*value.None); none {
		return false
	}
	_, literal := v.(*value.Str)
	return t.Kind == types.TypeApp || t.Kind == types.LitUnion && !literal && t.Elem != nil && t.Elem.Kind == types.TypeApp
}

// checkGoDecodedDependents is E8019 `DependentType` where gen/go's data loader cannot read a dependent value of a class it decodes (the plan's Decoded; CODEGEN.md §5.6; log-2026-09-25 "gen/go review round 1 calls").
func (s *stage) checkGoDecodedDependents(u *unit, es *emitSite) {
	pl := PlanGoNames(u.p, es.e)
	for _, class := range packageClasses(u.p) {
		if pl.Decoded(class) {
			s.reportDecodedDependents(u, es, class, goDependents)
		}
	}
}

// checkCppDependents is E8019 `DependentType` where gen/cpp refuses a dependent type: one every arm of which is Never, one with a branch into a load.defines table (its refStorage), and what its loader, which decodes every class, cannot read (CODEGEN.md §4.4, §5.6, §5.8).
func (s *stage) checkCppDependents(u *unit, es *emitSite) {
	for _, t := range u.p.Types {
		if d, ok := t.(*Dependent); ok && cppRefusesDependent(d) {
			u.reportGenConstruct(es, s.decls[d].span(), diag.KindDependentType)
		}
	}
	for _, class := range packageClasses(u.p) {
		s.reportDecodedDependents(u, es, class, cppDependents)
	}
}

// cppRefusesDependent reports a dependent type gen/cpp does not write: every arm Never (dependentNoBranch), or a branch into a load.defines table (refStorage).
func cppRefusesDependent(d *Dependent) bool {
	return len(d.Branches) == 0 || slices.ContainsFunc(d.Branches, func(b *Branch) bool { return definesRef(b.Type) })
}

// reportDecodedDependents is `DependentType` at each field and stored fn of class whose dependent value r's loader cannot read; a map is MapField's.
func (s *stage) reportDecodedDependents(u *unit, es *emitSite, class any, r dependentReader) {
	fields, fns := classBody(class)
	for _, f := range fields {
		if f.Input == nil && (!f.Optional || f.Type.Kind != types.Never) && !r.readsField(u.p.Name, fields, f) {
			u.reportGenConstruct(es, s.itemSpan(f, source.Span{}), diag.KindDependentType)
		}
	}
	for _, fn := range fns {
		if fn.Kind != FnTranslated && unreadDependent(&fn.Result) {
			u.reportGenConstruct(es, s.itemSpan(fn, source.Span{}), diag.KindDependentType)
		}
	}
}

// readsField reports that the loader reads field f of fields: no dependent type in it, or one application it holds whose discriminant it reads; a pairs field's slots are read without one.
func (r dependentReader) readsField(own string, fields []*Field, f *Field) bool {
	if f.Pairs != nil {
		return !slices.ContainsFunc(readFieldSites(f, source.Span{}), func(site typeSite) bool { return unreadDependent(site.t) })
	}
	app := &f.Type
	for r.throughLists && app.Kind == types.List && app.Elem != nil {
		app = app.Elem
	}
	if app.Kind == types.TypeApp {
		return r.disc(own, fields, app)
	}
	return !unreadDependent(&f.Type)
}

// unreadDependent reports a dependent type in t, a literal union's base included, that is not in a map (MapField's).
func unreadDependent(t *TypeRef) bool {
	return !decodedHolds(t, isMap) && typeHolds(t, func(t *TypeRef) bool { return t.Kind == types.TypeApp })
}

// goDiscRead reports that gen/go reads app's discriminant: an own dependent type with an enum discriminant, argument the whole of one required earlier field of the class that is the discriminant itself (gen/go dependentDisc; its decoder is unexported, so another package's is refused). An input field is refused deliberately: the loader never decodes its slot (log-2026-09-25, E8019 lift round 2).
func goDiscRead(own string, fields []*Field, app *TypeRef) bool {
	d, src := appSource(app)
	if d == nil || d.Pkg != own || d.Disc.Kind != types.Enum || len(d.DiscPath) > 0 || src.From != types.ArgField || len(src.WirePath) != 1 {
		return false
	}
	if _, ok := d.Disc.Named.(*Enum); !ok {
		return false
	}
	i := slices.IndexFunc(fields, func(f *Field) bool { return slices.Equal(f.WirePath, src.WirePath) })
	return i >= 0 && discField(fields[i], d)
}

// cppDiscRead reports that gen/cpp reads app's discriminant: the argument's then the match's path, through earlier fields of the class and of records held by value, none optional, an input or a ref, ending at the discriminant (gen/cpp discExpr). An input field is refused deliberately, as for gen/go (log-2026-09-25, E8019 lift round 2).
func cppDiscRead(_ string, fields []*Field, app *TypeRef) bool {
	d, src := appSource(app)
	if d == nil || src.From != types.ArgField {
		return false
	}
	segs := slices.Concat(src.WirePath, d.DiscPath)
	var last *Field
	for len(segs) > 0 {
		f := fieldAtWire(fields, segs)
		if f == nil || f.Optional || f.Input != nil || f.Type.Kind == types.Ref {
			return false
		}
		segs, last = segs[len(f.WirePath):], f
		fields = nil
		if rec, ok := f.Type.Named.(*Record); ok && f.Type.Kind == types.Record {
			fields = rec.Fields
		}
	}
	return last != nil && last.Type.Kind == d.Disc.Kind && last.Type.Named == d.Disc.Named
}

// appSource is the dependent type app applies and the source of its discriminant's argument; nil for a malformed application.
func appSource(app *TypeRef) (*Dependent, *Source) {
	d, ok := app.Named.(*Dependent)
	if !ok || d.Disc == nil || d.DiscParam < 0 || d.DiscParam >= len(app.Args) || app.Args[d.DiscParam] == nil {
		return nil, nil
	}
	return d, app.Args[d.DiscParam]
}

// discField reports a field that holds d's discriminant as the loader has decoded it: required, not an input, of the discriminant's type.
func discField(f *Field, d *Dependent) bool {
	return !f.Optional && f.Input == nil && f.Type.Kind == d.Disc.Kind && f.Type.Named == d.Disc.Named
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
