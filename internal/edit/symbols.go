package edit

import (
	"slices"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// symbolsIn is v, of declared type t or nil, in fr, each symbol a literal gave a dependent type as
// the member, case or key it names in the branch fr computes (DEP-02); was is the value the path
// held before the edit, or nil. Only what holds a symbol is copied.
func (fr depFrame) symbolsIn(v, was value.Value, t types.Type) (value.Value, error) {
	switch x := v.(type) {
	case *value.Symbol:
		if t == nil {
			t = x.T
		}
		return fr.symbol(x, was, t)
	case *value.Record:
		r, err := fr.recordSymbols(x, was, t)
		if err != nil {
			return nil, err
		}
		return r, nil
	case *value.List:
		return fr.listSymbols(x, was, t)
	case *value.Map:
		return fr.mapSymbols(x, was, t)
	case *value.Table:
		return fr.tableSymbols(x, was, t)
	}
	return v, nil
}

// symbol is s, of type t, in fr (log-2026-09-29 M4 B7-r3): one carried from was is left to E15;
// data a JSON source wrote, the path's symbol restated or a path key is kept where its branch is
// opaque (noted held by a strict frame); any other is resolved.
func (fr depFrame) symbol(s *value.Symbol, was value.Value, t types.Type) (value.Value, error) {
	w, isSymbol := was.(*value.Symbol)
	switch {
	case isSymbol && w == s:
		return s, nil
	case s.P != nil:
		return fr.keepData(s, t), nil
	case isSymbol && w.Name == s.Name && fr.kept != nil && fr.opaque(t):
		return fr.keepData(w, t), nil
	case fr.a != nil && fr.a.marks.pathKey(s) && fr.opaque(t):
		return s, nil // a path key read as the wire key the decoder keeps as a symbol (E25)
	}
	return fr.symbolIn(s, t)
}

// keepData is s, data as a JSON source wrote it, noted held where its branch is opaque.
func (fr depFrame) keepData(s *value.Symbol, t types.Type) *value.Symbol {
	if fr.kept != nil && fr.opaque(t) {
		fr.kept[s] = true
	}
	return s
}

// opaque reports t's branch Never or not computable here: where the decoder keeps a symbol.
func (fr depFrame) opaque(t types.Type) bool {
	et, ok := fr.branchFor(t)
	return !ok || et.Base().Kind() == types.Never
}

// branchFor is the type t computes in fr, through type applications and literal unions; false
// when it cannot be computed here, as the decoder cannot either.
func (fr depFrame) branchFor(t types.Type) (types.Type, bool) {
	for range maxLitDepth {
		switch b := present(t).Base().(type) {
		case *types.TypeAppType:
			br, inner, ok := fr.branch(b)
			if !ok {
				return nil, false
			}
			t, fr = br, inner
		case *types.LitUnionType:
			t = b.Of
		case *types.DepUnionType:
			return nil, false
		default:
			return present(t), true
		}
	}
	return nil, false
}

// symbolIn is s, a name the edit gives, as what it names in the branch t computes in fr: a
// member, case or key by its Canon name, a path key's also by its wire value (E25); s where the
// branch cannot be computed; else a *ValueError (V1), as a Set of the field alone refuses it.
func (fr depFrame) symbolIn(s *value.Symbol, t types.Type) (value.Value, error) {
	et, ok := fr.branchFor(t)
	switch {
	case !ok:
		return s, nil
	case et.Base().Kind() == types.Never:
		return nil, &ValueError{Expected: et.String(), Got: s.Name, Detail: detailNotNamed}
	}
	if v, found := nameValue(s.Name, et); found {
		return v, nil
	}
	if fr.a != nil && fr.a.marks.pathKey(s) {
		if v := textKeyValue(s.Name, et, true); v != nil {
			return v, nil
		}
	}
	return nil, &ValueError{Expected: et.String(), Got: s.Name, Detail: detailNotNamed}
}

// recordSymbols is r with its fields' symbols resolved in r's own frame (TYPES.md §11.1).
func (fr depFrame) recordSymbols(r *value.Record, was value.Value, t types.Type) (*value.Record, error) {
	in := fr.enter(r, t)
	fields := fieldsOf(r.T)
	w, _ := was.(*value.Record)
	var out []value.Value
	for i, v := range r.Fields {
		if v == nil || i >= len(fields) {
			continue
		}
		nv, err := in.symbolsIn(v, fieldBefore(w, r, i), fields[i].Type)
		if err != nil {
			return nil, err
		}
		out = replaced(out, r.Fields, i, nv)
	}
	if out == nil {
		return r, nil
	}
	return &value.Record{T: r.T, Fields: out, Set: r.Set, Ident: r.Ident, P: r.P}, nil
}

// fieldBefore is field i of r before the edit, in w: by index in a record of r's shape, else by
// name (a case a SetCase leaves); nil for none.
func fieldBefore(w, r *value.Record, i int) value.Value {
	if w == nil {
		return nil
	}
	if !sameShape(w.T, r.T) {
		i = fieldIndex(fieldsOf(w.T), fieldsOf(r.T)[i].Name)
	}
	if i < 0 || i >= len(w.Fields) {
		return nil
	}
	return w.Fields[i]
}

func (fr depFrame) listSymbols(l *value.List, was value.Value, t types.Type) (value.Value, error) {
	var et types.Type
	if lt, ok := declaredBase(t, l.T).(*types.ListType); ok {
		et = lt.Elem
	}
	var prev []value.Value
	if w, ok := was.(*value.List); ok {
		prev = w.Elems
	}
	var out []value.Value
	for i, e := range l.Elems {
		ne, err := fr.symbolsIn(e, elemAt(prev, i), et)
		if err != nil {
			return nil, err
		}
		out = replaced(out, l.Elems, i, ne)
	}
	if out == nil {
		return l, nil
	}
	return &value.List{T: l.T, Elems: out, P: l.P}, nil
}

// elemAt is vs[i], nil past its end.
func elemAt(vs []value.Value, i int) value.Value {
	if i >= len(vs) {
		return nil
	}
	return vs[i]
}

// mapSymbols is m with its keys' and values' symbols resolved, a dependent map's binder bound.
func (fr depFrame) mapSymbols(m *value.Map, was value.Value, t types.Type) (value.Value, error) {
	var kt, vt types.Type
	binder := ""
	switch mt := declaredBase(t, m.T).(type) {
	case *types.MapType:
		kt, vt = mt.Key, mt.Value
	case *types.DepMapType:
		vt, binder = mt.Value, mt.Binder
	}
	w, _ := was.(*value.Map)
	var keys, vals []value.Value
	for i := range m.Keys {
		wk, wv := keyedIn(w, m.Keys[i])
		k, err := fr.symbolsIn(m.Keys[i], wk, kt)
		if err != nil {
			return nil, err
		}
		vf := fr
		if binder != "" {
			vf = fr.bind(binder, k)
		}
		v, err := vf.symbolsIn(m.Vals[i], wv, vt)
		if err != nil {
			return nil, err
		}
		keys, vals = replaced(keys, m.Keys, i, k), replaced(vals, m.Vals, i, v)
	}
	if keys == nil && vals == nil {
		return m, nil
	}
	return &value.Map{T: m.T, Keys: orSame(keys, m.Keys), Vals: orSame(vals, m.Vals), P: m.P}, nil
}

// keyedIn is the key equal to k in w before the edit, and its value; nils for none.
func keyedIn(w *value.Map, k value.Value) (value.Value, value.Value) {
	if w == nil {
		return nil, nil
	}
	i := slices.IndexFunc(w.Keys, func(x value.Value) bool { return sameValue(x, k) })
	if i < 0 {
		return nil, nil
	}
	return w.Keys[i], w.Vals[i]
}

func (fr depFrame) tableSymbols(tb *value.Table, was value.Value, t types.Type) (value.Value, error) {
	var et types.Type
	if tt, ok := declaredBase(t, tb.T).(*types.TableType); ok {
		et = tt.Elem
	}
	w, _ := was.(*value.Table)
	var out []*value.Record
	for i, e := range tb.Entries {
		ne, err := fr.recordSymbols(e, entryIn(w, e), et)
		if err != nil {
			return nil, err
		}
		if ne != e && out == nil {
			out = slices.Clone(tb.Entries)
		}
		if out != nil {
			out[i] = ne
		}
	}
	if out == nil {
		return tb, nil
	}
	return &value.Table{T: tb.T, Entries: out, P: tb.P}, nil
}

// entryIn is the entry of w keyed like e before the edit, nil for none.
func entryIn(w *value.Table, e *value.Record) value.Value {
	if w == nil {
		return nil
	}
	i := slices.IndexFunc(w.Entries, func(x *value.Record) bool { return entryKey(x) == entryKey(e) })
	if i < 0 {
		return nil
	}
	return w.Entries[i]
}

// replaced is out, a copy of vs made at the first item that changed, with item i set to nv.
func replaced(out, vs []value.Value, i int, nv value.Value) []value.Value {
	if out == nil && nv == vs[i] {
		return nil
	}
	if out == nil {
		out = slices.Clone(vs)
	}
	out[i] = nv
	return out
}

// orSame is out, or vs when nothing in it changed.
func orSame(out, vs []value.Value) []value.Value {
	if out == nil {
		return vs
	}
	return out
}

// declaredBase is the base of t, a declared type (nil: own), without its optional layer.
func declaredBase(t, own types.Type) types.Type {
	if t == nil {
		t = own
	}
	if t == nil {
		return nil
	}
	return present(t).Base()
}
