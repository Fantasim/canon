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

// checkRepresentable is E8012: no emitted type, value, constant or stored fn holds a Range, a function type, `_` or a Never outside an optional field (CODEGEN.md §4.4); a define record or a table of one is added only when the package has a baked go emit, the one target known to refuse it (decision 180), so check fails wherever build would (decision 37).
func (s *stage) checkRepresentable(u *unit) {
	defines := bakedFor(u, TargetGo)
	s.eachOwnField(u, func(owner string, f *Field) {
		site := s.fieldSites[f]
		if what := unrepresentable(site.tf.Type, true, defines); what != nil {
			u.report(diag.E8012.At(site.span(), owner+qnameSep+f.Name, what))
		}
	})
	for _, v := range u.values {
		if what := unrepresentable(v.t, false, defines); what != nil {
			u.report(diag.E8012.At(v.span().span(), v.v.Name, what))
		}
	}
	for _, c := range u.consts {
		if what := unrepresentable(c.obj.Type(), false, defines); what != nil {
			u.report(diag.E8012.At(c.span(), c.c.Name, what))
		}
	}
	for _, site := range s.ownFns(u) {
		if site.fn.Kind == FnTranslated {
			continue
		}
		if what := unrepresentable(site.sig.Result, false, defines); what != nil {
			u.report(diag.E8012.At(site.span(), site.label, what))
		}
	}
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

// noWireForm reports a type holding a Range or a function type, through records and variants (WIRE.md §5.9: E8151; FINGERPRINT.md §8); a define record counts only when defines is set (decision 180, decision 37: check fails wherever build would).
func noWireForm(t types.Type, seen map[types.Type]bool, defines bool) bool {
	b := t.Base()
	if seen[b] {
		return false
	}
	seen[b] = true
	switch x := b.(type) {
	case *types.RecordType:
		return defines && x.Kind() == types.Define || fieldsWithoutWire(x.Fields, seen, defines)
	case *types.AppliedRecord:
		return noWireForm(x.Rec, seen, defines)
	case *types.VariantType:
		return casesWithoutWire(x.Cases, seen, defines)
	}
	if k := b.Kind(); k == types.Range || k == types.Func || defines && isDefineType(b) {
		return true
	}
	for _, sub := range subTypes(b) {
		if noWireForm(sub, seen, defines) {
			return true
		}
	}
	return false
}

func fieldsWithoutWire(fields []*types.Field, seen map[types.Type]bool, defines bool) bool {
	for _, f := range fields {
		if f.Input == nil && noWireForm(f.Type, seen, defines) {
			return true
		}
	}
	return false
}

func casesWithoutWire(cases []*types.CaseType, seen map[types.Type]bool, defines bool) bool {
	for _, c := range cases {
		if fieldsWithoutWire(c.Fields, seen, defines) {
			return true
		}
	}
	return false
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
