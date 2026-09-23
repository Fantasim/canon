package value

import (
	"strings"

	"github.com/fantasim/canonlang/internal/types"
)

// Record is a record or case value. T is a *types.RecordType, *types.CaseType or
// *types.AppliedRecord, possibly behind an alias or refinement.
type Record struct {
	T      types.Type
	Fields []Value   // declaration order; an absent optional holds a *None, an input field nil
	Set    []bool    // the field was written in the source (VM-06, API-06)
	Ident  *Identity // non-nil for table entries and keyed-list elements (TYP-02)
	P      *Prov
}

func (v *Record) Type() types.Type { return v.T }

func (v *Record) Prov() *Prov { return v.P }

// CanonText is Name{field: value, …}, inputs omitted; a case without fields is its name.
func (v *Record) CanonText() string {
	name, fields := shape(v.T)
	if len(fields) == 0 {
		if _, isCase := v.T.Base().(*types.CaseType); isCase {
			return name
		}
	}
	parts := make([]string, 0, len(fields))
	for i, f := range fields {
		if f.Input == nil {
			parts = append(parts, f.Name+textColon+nested(v.Fields[i]))
		}
	}
	return name + textOpenMap + strings.Join(parts, textSep) + textCloseMap
}

// equal compares the declaration, the case and every field; identities are ignored.
func (v *Record) equal(o Value) bool {
	w, ok := o.(*Record)
	if !ok || decl(v.T) != decl(w.T) || len(v.Fields) != len(w.Fields) {
		return false
	}
	for i, f := range v.Fields {
		if (f == nil) != (w.Fields[i] == nil) || f != nil && !Equal(f, w.Fields[i]) {
			return false
		}
	}
	return true
}

// shape is the unqualified name and the fields of a record, case or applied record type.
func shape(t types.Type) (string, []*types.Field) {
	switch d := decl(t).(type) {
	case *types.RecordType:
		return d.Name, d.Fields
	case *types.CaseType:
		return d.Name, d.Fields
	}
	return t.String(), nil
}

// decl is the declaration a record value's type names: arguments of R(args) are erased.
func decl(t types.Type) types.Type {
	if a, ok := t.Base().(*types.AppliedRecord); ok {
		return a.Rec
	}
	return t.Base()
}

// List is a plain or keyed list; the elements of a keyed list carry an Identity.
type List struct {
	T     types.Type
	Elems []Value
	P     *Prov
}

func (v *List) Type() types.Type { return v.T }

func (v *List) Prov() *Prov { return v.P }

func (v *List) CanonText() string {
	return textOpenList + nestedList(v.Elems) + textCloseList
}

func (v *List) equal(o Value) bool {
	w, ok := o.(*List)
	return ok && equalSlices(v.Elems, w.Elems)
}

// Map is a map value; Keys and Vals are parallel, in insertion order (TYPES.md §9.2).
type Map struct {
	T    types.Type
	Keys []Value
	Vals []Value
	P    *Prov
}

func (v *Map) Type() types.Type { return v.T }

func (v *Map) Prov() *Prov { return v.P }

func (v *Map) CanonText() string {
	parts := make([]string, len(v.Keys))
	for i, k := range v.Keys {
		parts[i] = nested(k) + textColon + nested(v.Vals[i])
	}
	return textOpenMap + strings.Join(parts, textSep) + textCloseMap
}

// Get is the value at key k, by value equality, and whether k is a key.
func (v *Map) Get(k Value) (Value, bool) {
	for i, key := range v.Keys {
		if Equal(key, k) {
			return v.Vals[i], true
		}
	}
	return nil, false
}

// equal is the same key set with equal values, order ignored (TYPES.md §7.5).
func (v *Map) equal(o Value) bool {
	w, ok := o.(*Map)
	if !ok || len(v.Keys) != len(w.Keys) {
		return false
	}
	for i, k := range v.Keys {
		if x, found := w.Get(k); !found || !Equal(v.Vals[i], x) {
			return false
		}
	}
	return true
}

// Table is a table value; every entry carries its Identity, key included, in entry order.
type Table struct {
	T       types.Type
	Entries []*Record
	P       *Prov
}

func (v *Table) Type() types.Type { return v.T }

func (v *Table) Prov() *Prov { return v.P }

func (v *Table) CanonText() string {
	parts := make([]string, len(v.Entries))
	for i, e := range v.Entries {
		parts[i] = e.Ident.Key.Text() + textColon + e.CanonText()
	}
	return textOpenMap + strings.Join(parts, textSep) + textCloseMap
}

// equal is the same keys in the same order and equal entries.
func (v *Table) equal(o Value) bool {
	w, ok := o.(*Table)
	if !ok || len(v.Entries) != len(w.Entries) {
		return false
	}
	for i, e := range v.Entries {
		if e.Ident.Key != w.Entries[i].Ident.Key || !e.equal(w.Entries[i]) {
			return false
		}
	}
	return true
}

// Ref is a ref value: a key of its type's target collection. Owner binds a ref resolved in
// an enclosing record to that record's instance (RES-03).
type Ref struct {
	T     types.Type
	Key   Key
	Owner *Record
	P     *Prov
}

func (v *Ref) Type() types.Type { return v.T }

func (v *Ref) Prov() *Prov { return v.P }

func (v *Ref) CanonText() string { return v.Key.Text() }

// identity is the entry the ref names: its target collection, owner and key.
func (v *Ref) identity() *Identity {
	r, _ := v.T.Base().(*types.RefType)
	return &Identity{Coll: r.Target, Owner: v.Owner, Key: v.Key}
}

// Pair is a pair value (TYPES.md §12.5).
type Pair struct {
	T    types.Type
	A, B Value
	P    *Prov
}

func (v *Pair) Type() types.Type { return v.T }

func (v *Pair) Prov() *Prov { return v.P }

func (v *Pair) CanonText() string {
	return textOpen + nested(v.A) + textSep + nested(v.B) + textClose
}

func (v *Pair) equal(o Value) bool {
	w, ok := o.(*Pair)
	return ok && Equal(v.A, w.A) && Equal(v.B, w.B)
}
