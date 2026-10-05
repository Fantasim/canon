package ir

import (
	"slices"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

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
