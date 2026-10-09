package edit

import (
	"strings"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// readRecord is `.f` on a record or a variant's current case, else a pseudo-field (API.md P3):
// a field of that name wins over the pseudo-field (RES-08).
func readRecord(rec *value.Record, seg Seg) (read, error) {
	if seg.Kind != SegField {
		return read{}, ErrBadPath // P5
	}
	fields := fieldsOf(rec.T)
	if i := fieldIndex(fields, seg.Name); i >= 0 {
		return read{v: rec.Fields[i], typ: fields[i].Type, seg: seg}, nil
	}
	if v, t, ok := pseudo(rec, seg.Name); ok {
		return read{v: v, typ: t, seg: seg}, nil
	}
	return read{}, ErrNoPath
}

// pseudo is `.id` and `.retired` of a table entry, `.kind` of a variant (API.md P3).
func pseudo(rec *value.Record, name string) (value.Value, types.Type, bool) {
	entry := rec.Ident != nil && (rec.Ident.Coll == nil || rec.Ident.Coll.KeyedBy == nil)
	switch {
	case name == pseudoID && entry:
		if rec.Ident.Key.IsInt {
			return &value.Int{V: rec.Ident.Key.I, T: types.IntType, P: rec.P}, types.IntType, true
		}
		return &value.Str{V: rec.Ident.Key.S, T: types.StringType, P: rec.P}, types.StringType, true
	case name == pseudoRetired && entry:
		return &value.Bool{V: rec.Ident.Retired, P: rec.P}, types.BoolType, true
	case name == pseudoKind:
		if c, ok := rec.T.Base().(*types.CaseType); ok {
			k := &types.VariantKindType{Variant: c.Variant}
			return &value.CaseKind{T: k, Index: c.Index, P: rec.P}, k, true
		}
	}
	return nil, nil, false
}

// readList is `[n]` on a plain list, `[k]` and `.k` by key on a keyed list (P1), `[#n]` on both.
func readList(l *value.List, ct types.Type, seg Seg) (read, error) {
	lt, ok := ct.Base().(*types.ListType)
	if !ok {
		return read{}, ErrBadPath
	}
	if seg.Kind == SegPos {
		return atPosition(l.Elems, lt.Elem, seg.Pos, func(i int) Seg { return listSeg(lt, l.Elems[i], i) })
	}
	if lt.KeyedBy == nil {
		return plainIndex(l, lt, seg)
	}
	match, err := segKey(seg, lt.KeyedBy.Type)
	if err != nil {
		return read{}, err
	}
	for _, e := range l.Elems {
		if k := keyField(e, lt.KeyedBy); k != nil && match(k) {
			return read{v: e, typ: lt.Elem, seg: keySeg(k, lt.KeyedBy.Type)}, nil
		}
	}
	return read{}, ErrNoPath
}

// plainIndex is `[k]` on a plain list: k is digits, the element's index (§6.2).
func plainIndex(l *value.List, lt *types.ListType, seg Seg) (read, error) {
	if seg.Kind != SegKey || seg.Key.Kind != KeyInt || seg.Key.Int < 0 || strings.HasPrefix(seg.Key.Raw, string(minus)) {
		return read{}, ErrBadPath
	}
	if seg.Key.Int >= int64(len(l.Elems)) {
		return read{}, ErrNoPath // P4
	}
	i := int(seg.Key.Int)
	return read{v: l.Elems[i], typ: lt.Elem, seg: intSeg(int64(i))}, nil
}

// readTable is `.k`, `[k]` (a word or a JSON string) or `[#n]` on a table (§6.2).
func readTable(t *value.Table, ct types.Type, seg Seg, keys *tableKeys) (read, error) {
	elem := tableElem(ct, t)
	if seg.Kind == SegPos {
		return atPosition(t.Entries, elem, seg.Pos, func(i int) Seg { return entrySeg(t.Entries[i].Ident.Key) })
	}
	name := seg.Name
	if seg.Kind == SegKey {
		if seg.Key.Kind == KeyInt {
			return read{}, ErrBadPath
		}
		name = seg.Key.Text
	}
	i, ok := keys.find(t, name)
	if !ok {
		return read{}, ErrNoPath
	}
	e := t.Entries[i]
	return read{v: e, typ: elem, seg: entrySeg(e.Ident.Key)}, nil
}

// tableElem is the static entry type of a table: the container's, else the value's own.
func tableElem(ct types.Type, t *value.Table) types.Type {
	if tt, ok := ct.Base().(*types.TableType); ok {
		return tt.Elem
	}
	if tt, ok := t.T.Base().(*types.TableType); ok {
		return tt.Elem
	}
	return nil
}

// readMap is `[k]` by key (P2) or `[#n]` on a map, a dependent map keyed by refs (TYPES.md §11.5).
func readMap(m *value.Map, ct types.Type, seg Seg) (read, error) {
	kt, vt, ok := mapTypes(ct)
	if !ok {
		return read{}, ErrBadPath
	}
	switch seg.Kind {
	case SegPos:
		return atPosition(m.Vals, vt, seg.Pos, func(i int) Seg { return keySeg(m.Keys[i], kt) })
	case SegField:
		return read{}, ErrBadPath // `.name` is not a map key (P5)
	case SegKey:
	}
	match, err := segKey(seg, kt)
	if err != nil {
		return read{}, err
	}
	for i, k := range m.Keys {
		if match(k) {
			return read{v: m.Vals[i], typ: vt, seg: keySeg(k, kt)}, nil
		}
	}
	return read{}, ErrNoPath
}

// mapTypes are the key and value types of a map type; a dependent map is keyed by refs (TYPES.md §11.5).
func mapTypes(t types.Type) (key, val types.Type, ok bool) {
	switch b := baseOf(t).(type) {
	case *types.MapType:
		return b.Key, b.Value, true
	case *types.DepMapType:
		return &types.RefType{Target: b.Coll}, b.Value, true
	}
	return nil, nil, false
}

// segKey is the matcher of a key segment; `.name` is not a map key, a key of the wrong form
// is not a key of that type (P5).
func segKey(seg Seg, kt types.Type) (keyMatch, error) {
	lit := seg.Key
	switch seg.Kind {
	case SegField:
		lit = KeyLit{Kind: KeyWord, Text: seg.Name, Raw: seg.Name}
	case SegKey:
	default:
		return nil, ErrBadPath
	}
	match, ok := keyMatcher(lit, kt)
	if !ok {
		return nil, ErrBadPath
	}
	return match, nil
}

// atPosition is `[#n]`: the element at position n, ErrNoPath past the end (P4).
func atPosition[V value.Value](elems []V, typ types.Type, n int, seg func(int) Seg) (read, error) {
	switch {
	case n < 0:
		return read{}, ErrBadPath
	case n >= len(elems):
		return read{}, ErrNoPath
	}
	return read{v: elems[n], typ: typ, seg: seg(n)}, nil
}

// listSeg is an element's canonical segment: its key on a keyed list, its index otherwise (P8).
func listSeg(lt *types.ListType, e value.Value, i int) Seg {
	if lt.KeyedBy != nil {
		if k := keyField(e, lt.KeyedBy); k != nil {
			return keySeg(k, lt.KeyedBy.Type)
		}
	}
	return intSeg(int64(i))
}

// keyField is the key field's value of a keyed-list element, nil when it has none.
func keyField(e value.Value, key *types.Field) value.Value {
	rec, ok := e.(*value.Record)
	if !ok || key.Index >= len(rec.Fields) {
		return nil
	}
	return rec.Fields[key.Index]
}
