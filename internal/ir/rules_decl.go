package ir

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
)

func (site *fnSite) span() source.Span { return site.obj.File().Span(site.decl.Name) }

// ownFns are the export fns u declares: its package fns, then its public types' methods.
func (s *stage) ownFns(u *unit) []*fnSite {
	out := append([]*fnSite(nil), u.fns...)
	add := func(fns []*ExportFn) {
		for _, fn := range fns {
			if site := s.fnObjs[fn]; site != nil {
				out = append(out, site)
			}
		}
	}
	for _, t := range u.p.Types {
		switch x := t.(type) {
		case *Record:
			add(x.Methods)
		case *Variant:
			for _, c := range x.Cases {
				add(c.Methods)
			}
		}
	}
	return out
}

// eachOwnField calls f with each field of u's public records and cases and its owner's name.
func (s *stage) eachOwnField(u *unit, f func(owner string, fd *Field)) {
	for _, t := range u.p.Types {
		switch x := t.(type) {
		case *Record:
			eachField(x.Name, x.Fields, f)
		case *Variant:
			for _, c := range x.Cases {
				eachField(x.Name+qnameSep+c.Name, c.Fields, f)
			}
		}
	}
}

func eachField(owner string, fields []*Field, f func(owner string, fd *Field)) {
	for _, fd := range fields {
		f(owner, fd)
	}
}

// checkFnSignatures is E9003 (an optional parameter) and E9002 (a lookup table over maxCells cells) on u's export fns (CONFORMANCE.md §2.1, CODEGEN.md §5.10).
func (s *stage) checkFnSignatures(u *unit) {
	for _, site := range s.ownFns(u) {
		for i, p := range site.sig.Params {
			if p.Base().Kind() == types.Optional && i < len(site.decl.Params) {
				u.report(diag.E9003.At(site.obj.File().Span(site.decl.Params[i].Name), site.fn.Params[i].Name, site.label))
			}
		}
		if site.fn.Kind != FnLookup {
			continue
		}
		if _, n, ok := s.domains(site); ok && n > maxCells {
			u.report(diag.E9002.At(site.span(), site.label, n))
		}
	}
}

// checkOrderedCodes is E8010: an ordered enum's codes increase in declaration order, since generated `<` compares codes (CODEGEN.md §5.2).
func (s *stage) checkOrderedCodes(u *unit) {
	for _, t := range u.p.Types {
		e, ok := t.(*Enum)
		if !ok || !e.Ordered || e.Codes == nil {
			continue
		}
		for i := 1; i < len(e.Members); i++ {
			if e.Members[i].Code <= e.Members[i-1].Code {
				u.report(diag.E8010.At(s.decls[e].span(), e.Name))
				break
			}
		}
	}
}

// checkRepresentable is E8012: no emitted type, value, constant or stored fn holds a Range, a function type, `_` or a Never outside an optional field (CODEGEN.md §4.4); a value counts only where a code emit writes it (emittedValues); a define record or a table of one is added for a type or field when the package has a baked go emit or a ts emit, and for a value when one of them selects it, since neither baked go nor gen/ts can represent one (decisions 180, 194: per emit; DECISIONS 278 for ts), so check fails wherever build would (decision 37). Data and embedded modes refuse one through their values' fingerprints (checkFingerprinted).
func (s *stage) checkRepresentable(u *unit) {
	defines := bakedFor(u, TargetGo) || hasTarget(u, TargetTS)
	s.eachOwnField(u, func(owner string, f *Field) {
		site := s.fieldSites[f]
		if what := unrepresentable(site.tf.Type, true, defines); what != nil {
			u.report(unrepresentableFinding(site.span(), owner+qnameSep+f.Name, what))
		}
	})
	for _, v := range emittedValues(u) {
		if what := unrepresentable(v.t, false, definesRefused(u, v)); what != nil {
			u.report(unrepresentableFinding(v.span().span(), v.v.Name, what))
		}
	}
	for _, c := range u.consts {
		if what := unrepresentable(c.obj.Type(), false, defines); what != nil {
			u.report(unrepresentableFinding(c.span(), c.c.Name, what))
		}
	}
	for _, site := range s.ownFns(u) {
		if site.fn.Kind == FnTranslated {
			continue
		}
		if what := unrepresentable(site.sig.Result, false, defines); what != nil {
			u.report(unrepresentableFinding(site.span(), site.label, what))
		}
	}
}

// unrepresentableFinding is E8012 `define` for a load.defines table or record, else the general variant.
func unrepresentableFinding(span source.Span, name string, what types.Type) *diag.Builder {
	if isDefineType(what.Base()) {
		return diag.E8012.AtDefine(span, name)
	}
	return diag.E8012.AtType(span, name, what)
}

// unrepresentable is the first part of t generated code cannot hold; a field's own `Never?` is
// not emitted at all, and a define record or a table of one counts only when defines is set.
// Named types are judged at their own declarations.
func unrepresentable(t types.Type, field, defines bool) types.Type {
	b := t.Base()
	if o, ok := b.(*types.OptionalType); ok && field && o.Elem.Base().Kind() == types.Never {
		return nil
	}
	switch {
	case isUniversallyUnrepresentable(b.Kind()):
		return t
	case defines && isDefineType(b):
		return t
	}
	for _, sub := range subTypes(b) {
		if what := unrepresentable(sub, false, defines); what != nil {
			return what
		}
	}
	return nil
}

// isUniversallyUnrepresentable is CODEGEN.md §4.4's fixed list: no target can hold these.
func isUniversallyUnrepresentable(k types.Kind) bool {
	return k == types.Range || k == types.Func || k == types.Any || k == types.Never
}

// isDefineType is a define record, or a table of one (decision 180).
func isDefineType(b types.Type) bool {
	if b.Kind() == types.Define {
		return true
	}
	t, ok := b.(*types.TableType)
	return ok && t.Elem.Base().Kind() == types.Define
}

// subTypes are the types a composite type holds by value; a ref holds only a key.
func subTypes(t types.Type) []types.Type {
	switch x := t.(type) {
	case *types.OptionalType:
		return []types.Type{x.Elem}
	case *types.ListType:
		return []types.Type{x.Elem}
	case *types.MapType:
		return []types.Type{x.Key, x.Value}
	case *types.DepMapType:
		return []types.Type{x.Value}
	case *types.TableType:
		return []types.Type{x.Elem}
	case *types.LitUnionType:
		return []types.Type{x.Of}
	}
	return nil
}

// wireFind is the first type of t, through records, variants and their fields (inputs excluded) and composites, for which bad holds (WIRE.md §5.9, FINGERPRINT.md §8), or nil.
func wireFind(t types.Type, seen map[types.Type]bool, bad func(types.Type) bool) types.Type {
	b := t.Base()
	if seen[b] {
		return nil
	}
	seen[b] = true
	if bad(b) {
		return t
	}
	for _, sub := range wireParts(b) {
		if found := wireFind(sub, seen, bad); found != nil {
			return found
		}
	}
	return nil
}

// wireParts are the types a type's wire form holds: a record's or case's fields but inputs, an applied record's record, a variant's cases, a composite's parts.
func wireParts(b types.Type) []types.Type {
	switch x := b.(type) {
	case *types.RecordType:
		return fieldTypes(nil, x.Fields)
	case *types.AppliedRecord:
		return []types.Type{x.Rec}
	case *types.CaseType:
		return fieldTypes(nil, x.Fields)
	case *types.VariantType:
		var out []types.Type
		for _, c := range x.Cases {
			out = fieldTypes(out, c.Fields)
		}
		return out
	}
	return subTypes(b)
}

// fieldTypes appends the types of fields that are not inputs to out.
func fieldTypes(out []types.Type, fields []*types.Field) []types.Type {
	for _, f := range fields {
		if f.Input == nil {
			out = append(out, f.Type)
		}
	}
	return out
}

// noWire is a type with no wire form or no fingerprint: a Range, a function type, a define record or a table of one (WIRE.md §5.9; canon-fp has no Define form, decisions 126, 194).
func noWire(b types.Type) bool {
	k := b.Kind()
	return k == types.Range || k == types.Func || isDefineType(b)
}

// checkBranches is E8017: a dependent type's branches are scalars, String, enums or refs (CODEGEN.md §5.6).
func (s *stage) checkBranches(u *unit) {
	for _, t := range u.p.Types {
		d, ok := t.(*Dependent)
		if !ok {
			continue
		}
		for _, br := range d.Branches {
			if !branchKinds[br.Type.Kind] {
				u.report(diag.E8017.At(s.decls[d].span(), br.Name, d.Name, s.branchTypes[br]))
			}
		}
	}
}
