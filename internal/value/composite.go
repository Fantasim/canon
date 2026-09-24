package value

import "github.com/fantasim/canonlang/internal/types"

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

func (v *Record) CanonText() string { return render(v) }

// layout is Name{field: value, …}, inputs omitted; a case without fields is its name.
func (v *Record) layout() []piece {
	name, fields := shape(v.T)
	if len(fields) == 0 {
		if _, isCase := v.T.Base().(*types.CaseType); isCase {
			return []piece{{lit: name}}
		}
	}
	out := []piece{{lit: name + textOpenMap}}
	sep := ""
	for i, f := range fields {
		if f.Input == nil {
			out = append(out, piece{lit: sep + f.Name + textColon}, piece{v: v.Fields[i]})
			sep = textSep
		}
	}
	return append(out, piece{lit: textCloseMap})
}

// match compares the declaration, the case and which fields are set; identities are ignored.
func (v *Record) match(o Value, push func(a, b Value, plain bool)) bool {
	w, ok := o.(*Record)
	if !ok || decl(v.T) != decl(w.T) || len(v.Fields) != len(w.Fields) {
		return false
	}
	for i, f := range v.Fields {
		if (f == nil) != (w.Fields[i] == nil) {
			return false
		}
		if f != nil {
			push(f, w.Fields[i], false)
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

func (v *List) CanonText() string { return render(v) }

func (v *List) layout() []piece {
	return enclosed(textOpenList, textCloseList, v.Elems)
}

func (v *List) match(o Value, push func(a, b Value, plain bool)) bool {
	w, ok := o.(*List)
	return ok && pushSlices(v.Elems, w.Elems, push)
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

func (v *Map) CanonText() string { return render(v) }

func (v *Map) layout() []piece {
	out := []piece{{lit: textOpenMap}}
	for i, k := range v.Keys {
		if i > 0 {
			out = append(out, piece{lit: textSep})
		}
		out = append(out, piece{v: k}, piece{lit: textColon}, piece{v: v.Vals[i]})
	}
	return append(out, piece{lit: textCloseMap})
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

// match is the same key set, order ignored (TYPES.md §7.5); the values are pushed.
func (v *Map) match(o Value, push func(a, b Value, plain bool)) bool {
	w, ok := o.(*Map)
	if !ok || len(v.Keys) != len(w.Keys) {
		return false
	}
	for i, k := range v.Keys {
		x, found := w.Get(k)
		if !found {
			return false
		}
		push(v.Vals[i], x, false)
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

func (v *Table) CanonText() string { return render(v) }

func (v *Table) layout() []piece {
	out := []piece{{lit: textOpenMap}}
	sep := ""
	for _, e := range v.Entries {
		out = append(out, piece{lit: sep + e.Ident.Key.Text() + textColon}, piece{v: e})
		sep = textSep
	}
	return append(out, piece{lit: textCloseMap})
}

// match is the same keys in the same order; the entries are pushed, compared field-wise.
func (v *Table) match(o Value, push func(a, b Value, plain bool)) bool {
	w, ok := o.(*Table)
	if !ok || len(v.Entries) != len(w.Entries) {
		return false
	}
	for i, e := range v.Entries {
		if e.Ident.Key != w.Entries[i].Ident.Key {
			return false
		}
		push(e, w.Entries[i], true)
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

func (v *Pair) CanonText() string { return render(v) }

func (v *Pair) layout() []piece {
	return []piece{{lit: textOpen}, {v: v.A}, {lit: textSep}, {v: v.B}, {lit: textClose}}
}

func (v *Pair) match(o Value, push func(a, b Value, plain bool)) bool {
	w, ok := o.(*Pair)
	if !ok {
		return false
	}
	push(v.A, w.A, false)
	push(v.B, w.B, false)
	return true
}
