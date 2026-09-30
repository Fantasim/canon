package eval

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval/std"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// setElement replaces an element; only a last segment adds, a table entry (EVALUATION.md §9.3).
func (r *run) setElement(cur value.Value, t types.Type, segs []*syntax.AmendSegment, m *amending) value.Value {
	seg := segs[0]
	elems := std.Elems(cur)
	i, key, ok := r.slotOf(cur, seg, m)
	if !ok {
		return nil
	}
	if i < 0 {
		return r.addEntry(cur, t, segs, m, key)
	}
	nv := r.next(elems[i], elementType(t), segs, m, site{})
	if nv == nil {
		return nil
	}
	return r.copied(cur, r.ev.replaced(cur, i, nil, r.ev.withIdent(nv, elems[i])))
}

// slotOf is the element a segment names: its index, -1 for a key the collection lacks.
func (r *run) slotOf(cur value.Value, seg *syntax.AmendSegment, m *amending) (int, value.Value, bool) {
	elems := std.Elems(cur)
	if seg.Position != nil {
		return r.inRange(seg, m, seg.Position.Value.Int64(), len(elems), nil)
	}
	key := r.keyOf(seg, m)
	if key == nil {
		return 0, nil, false
	}
	if n, isInt := key.(*value.Int); isInt && isPlainList(cur) {
		return r.inRange(seg, m, n.V, len(elems), key)
	}
	k, _ := std.KeyOf(key)
	for i, e := range elems {
		if rec, isRec := e.(*value.Record); isRec && rec.Ident != nil && rec.Ident.Key == k {
			return i, key, true
		}
	}
	return -1, key, true
}

// inRange is position n of a sequence of length size, negative from the end; E1905 outside,
// unless the path is only being located.
func (r *run) inRange(seg *syntax.AmendSegment, m *amending, n int64, size int, key value.Value) (int, value.Value, bool) {
	i := n
	if i < 0 && key != nil {
		i += int64(size)
	}
	if i < 0 || i >= int64(size) {
		if !m.quiet {
			r.fail(diag.E1905.AtIndex(r.span(seg), m.path, n))
		}
		return 0, nil, false
	}
	return int(i), key, true
}

// keyOf is the key a segment writes, evaluated once per amendment (locate, then the replace).
func (r *run) keyOf(seg *syntax.AmendSegment, m *amending) value.Value {
	if k, ok := m.keys[seg]; ok {
		return k
	}
	k := r.segmentKey(seg)
	m.keys[seg] = k
	return k
}

// segmentKey is the key a segment writes: a word, or an expression in the layer's scope.
func (r *run) segmentKey(seg *syntax.AmendSegment) value.Value {
	if seg.Name != nil {
		return &value.Str{V: seg.Name.Name, T: types.StringType, P: r.prov(seg, value.ProvLayer)}
	}
	return r.eval(seg.Key)
}

// addEntry adds a new entry at the end of a table: a stable table takes none (LOCK.md §6.1).
func (r *run) addEntry(cur value.Value, t types.Type, segs []*syntax.AmendSegment, m *amending, key value.Value) value.Value {
	tbl, isTable := cur.(*value.Table)
	tt, _ := t.Base().(*types.TableType)
	if !isTable || len(segs) > 1 || tt == nil {
		r.fail(diag.E1905.AtKey(r.span(segs[0]), m.path, key))
		return nil
	}
	if tt.Stable {
		return r.forbid(m, m.root.Name(), "")
	}
	rec, ok := r.store(m.rhs, tt.Elem, site{}, m.pathAfter(len(m.a.Path))).(*value.Record)
	if !ok {
		r.bug(segs[0])
		return nil
	}
	k, _ := std.KeyOf(key)
	added := r.ev.withIdentity(rec, &value.Identity{Coll: tableColl(tbl, r.ev.letColl(m.root)), Key: k})
	entries := append(append([]*value.Record(nil), tbl.Entries...), added)
	return r.copied(tbl, &value.Table{T: tbl.T, Entries: entries, P: tbl.P})
}

// tableColl is the collection a table's entries belong to.
func tableColl(t *value.Table, fallback *types.Collection) *types.Collection {
	if len(t.Entries) > 0 && t.Entries[0].Ident != nil {
		return t.Entries[0].Ident.Coll
	}
	return fallback
}

// setMapKey replaces a map key's value, or adds it when last (EVALUATION.md §9.3).
func (r *run) setMapKey(mp *value.Map, t types.Type, segs []*syntax.AmendSegment, m *amending) value.Value {
	seg := segs[0]
	i, key, ok := r.mapSlot(mp, seg, m)
	if !ok {
		return nil
	}
	vt := types.AnyType
	if mt, isMap := t.Base().(*types.MapType); isMap {
		vt = mt.Value
	}
	if i < 0 && len(segs) > 1 {
		r.fail(diag.E1905.AtKey(r.span(seg), m.path, key))
		return nil
	}
	var cur value.Value
	if i >= 0 {
		cur = mp.Vals[i]
	} else if p := key.Prov(); p != nil {
		key = r.ev.reprov(key, &value.Prov{Kind: value.ProvLayer, Span: p.Span, Layer: m.layer}, true)
	}
	nv := r.next(cur, vt, segs, m, site{})
	if nv == nil {
		return nil
	}
	return r.copied(mp, r.ev.replaced(mp, i, key, nv))
}

// mapSlot is the entry a segment names: a position, or a key (-1 when absent).
func (r *run) mapSlot(mp *value.Map, seg *syntax.AmendSegment, m *amending) (int, value.Value, bool) {
	if seg.Position != nil {
		i := seg.Position.Value.Int64()
		if i < 0 || i >= int64(len(mp.Keys)) {
			if !m.quiet {
				r.fail(diag.E1905.AtIndex(r.span(seg), m.path, i))
			}
			return 0, nil, false
		}
		return int(i), mp.Keys[i], true
	}
	key := r.boundKey(r.keyOf(seg, m), m)
	if key == nil {
		return 0, nil, false
	}
	return r.ev.keyAt(mp, key), key, true
}

func keyPosition(m *value.Map, k value.Value) int {
	for i, key := range m.Keys {
		if value.Equal(key, k) {
			return i
		}
	}
	return -1
}
