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
		return fr.symbolIn(x, t, fr.isHeld(x, was), 0)
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

// isHeld reports s as data rather than a name the edit introduces: any symbol outside a strict
// frame, one read from a JSON value (DECISIONS 175), or the symbol the path held before the edit.
func (fr depFrame) isHeld(s *value.Symbol, was value.Value) bool {
	w, same := was.(*value.Symbol)
	return fr.kept == nil || s.P != nil || same && w.Name == s.Name
}

// keep is s, kept as written: noted in a strict frame when held (log-2026-09-29 M4 B7-r2).
func (fr depFrame) keep(s *value.Symbol, held bool) *value.Symbol {
	if held && fr.kept != nil {
		fr.kept[s] = true
	}
	return s
}

// symbolIn is s as what it names in the branch t computes in fr; s, written as the decoder reads
// it (DECISIONS 175), when that branch is unknown here or is Never and s held; else a *ValueError
// (V1), as a Set of the field alone refuses a name its computed type lacks.
func (fr depFrame) symbolIn(s *value.Symbol, t types.Type, held bool, depth int) (value.Value, error) {
	if t == nil || depth > maxLitDepth {
		return s, nil
	}
	switch b := present(t).Base().(type) {
	case *types.TypeAppType:
		br, inner, ok := fr.branch(b)
		if !ok {
			return fr.keep(s, held), nil
		}
		return inner.symbolIn(s, br, held, depth+1)
	case *types.LitUnionType:
		return fr.symbolIn(s, b.Of, held, depth+1)
	case *types.DepUnionType:
		return fr.keep(s, held), nil
	}
	et := present(t)
	if et.Base().Kind() == types.Never && held {
		return fr.keep(s, held), nil
	}
	if v, ok := nameValue(s.Name, et); ok {
		return v, nil
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

// fieldBefore is field i of w, a record of r's shape, before the edit; nil for none.
func fieldBefore(w, r *value.Record, i int) value.Value {
	if w == nil || !sameShape(w.T, r.T) || i >= len(w.Fields) {
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
