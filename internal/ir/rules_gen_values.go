package ir

import (
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// checkConstLiterals is E8019 `CrossPackageBakedValue`: gen/go writes a list or map constant as a literal in every mode (CODEGEN.md §5.1).
func (s *stage) checkConstLiterals(u *unit, es *emitSite) {
	for _, c := range u.consts {
		if bakesForeign(u.p.Name, &c.c.Type, c.c.V) {
			u.reportGenConstruct(es, c.span(), diag.KindCrossPackageBakedValue)
		}
	}
}

// checkBakedLiterals is E8019 `CrossPackageBakedValue` for baked literals: a selected value, and a stored fn's results once, at the fn (CODEGEN.md §5.9).
func (s *stage) checkBakedLiterals(u *unit, es *emitSite) {
	own := u.p.Name
	for _, v := range selectedValues(u, es.e) {
		if !foreignTable(own, v.v.Type) && bakesForeign(own, &v.v.Type, v.v.V) {
			u.reportGenConstruct(es, v.span().span(), diag.KindCrossPackageBakedValue)
		}
	}
	for _, site := range s.ownFns(u) {
		fn := site.fn
		results := func(r value.Value) bool { return bakesForeign(own, &fn.Result, r) }
		if fn.Kind != FnTranslated && slices.ContainsFunc(storedResults(fn), results) {
			u.reportGenConstruct(es, site.span(), diag.KindCrossPackageBakedValue)
		}
	}
}

// storedResults are every result baked Go writes for a stored fn: its value or cells, per receiver for a method.
func storedResults(fn *ExportFn) []value.Value {
	out := cellsOf(fn.Table)
	if fn.Value != nil {
		out = append(out, fn.Value)
	}
	for _, in := range fn.Instances {
		out = append(append(out, cellsOf(in.Table)...), in.Result)
	}
	return out
}

func cellsOf(t *LookupTable) []value.Value {
	if t == nil {
		return nil
	}
	return t.Cells
}

// bakesForeign reports that the Go literal of v, of type t, writes a record, variant or case of a package other than own; an own record's fns' results are reported at the fn.
func bakesForeign(own string, t *TypeRef, v value.Value) bool {
	return literalHolds(t, v, func(t *TypeRef, v value.Value) bool {
		_, rec := v.(*value.Record)
		return rec && recordKinds[t.Kind] && t.Named != nil && pkgOf(t.Named) != own
	})
}

// literalHolds reports a part of the Go literal of v, of type t, that bad accepts, v itself or its elements, keys, map values and fields: none is written for a `none` or an empty list.
func literalHolds(t *TypeRef, v value.Value, bad func(*TypeRef, value.Value) bool) bool {
	if t == nil || v == nil {
		return false
	}
	if t.Kind == types.Optional {
		return literalHolds(t.Elem, v, bad)
	}
	if bad(t, v) {
		return true
	}
	holds := func(t *TypeRef) func(value.Value) bool {
		return func(e value.Value) bool { return literalHolds(t, e, bad) }
	}
	switch x := v.(type) {
	case *value.Record:
		return recordHolds(t, x, bad)
	case *value.List:
		return slices.ContainsFunc(x.Elems, holds(t.Elem))
	case *value.Table:
		return slices.ContainsFunc(x.Entries, func(e *value.Record) bool { return literalHolds(t.Elem, e, bad) })
	case *value.Map:
		return slices.ContainsFunc(x.Keys, holds(t.Key)) || slices.ContainsFunc(x.Vals, holds(t.Elem))
	default:
		return false
	}
}

// recordHolds reports a field value of a record value that literalHolds accepts.
func recordHolds(t *TypeRef, r *value.Record, bad func(*TypeRef, value.Value) bool) bool {
	if !recordKinds[t.Kind] || t.Named == nil {
		return false
	}
	decl := ownFields(r.T)
	for _, f := range bodyFields(t, r) {
		i := slices.IndexFunc(decl, func(d *types.Field) bool { return d.Name == f.Name })
		if i >= 0 && i < len(r.Fields) && literalHolds(&f.Type, r.Fields[i], bad) {
			return true
		}
	}
	return false
}

// bodyFields are the IR fields of a record value of type t: its record's, or its case's.
func bodyFields(t *TypeRef, r *value.Record) []*Field {
	switch n := t.Named.(type) {
	case *Record:
		return n.Fields
	case *Variant:
		if r.T == nil {
			return nil
		}
		if ct, ok := r.T.Base().(*types.CaseType); ok && ct.Index >= 0 && ct.Index < len(n.Cases) {
			return n.Cases[ct.Index].Fields
		}
	}
	return nil
}
