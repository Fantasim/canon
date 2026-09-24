package eval

import (
	"github.com/fantasim/canonlang/internal/eval/std"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

type bindMode uint8

// copiedFrom records that cp is rec's instance; callers pass only copies no two values diverge
// in: one sharing rec's fields, or one of a value only this let's amendments hold.
func (e *Evaluator) copiedFrom(rec, cp *value.Record) {
	if rec != cp && len(e.owned(cp.T)) > 0 {
		e.origin[cp] = rec
	}
}

// instance is the record rec copies, through every copy: the instance rec is. The copies met
// are then recorded as copies of it, so a chain of amendments is walked once.
func (e *Evaluator) instance(rec *value.Record) *value.Record {
	root := rec
	for next := e.originOf(root); next != nil; next = e.originOf(root) {
		root = next
	}
	for rec != root {
		next := e.originOf(rec)
		e.origin[rec] = root
		rec = next
	}
	return root
}

// originOf is the record rec copies, nil for none; a vector's copies of its parent's values are its own.
func (e *Evaluator) originOf(rec *value.Record) *value.Record {
	if o, ok := e.origin[rec]; ok {
		return o
	}
	if e.parent != nil {
		return e.parent.originOf(rec)
	}
	return nil
}

// transfer makes cp, the binder's copy of rec, the owner its entries and refs name (EVALUATION.md §3.4, §4.2).
func (e *Evaluator) transfer(rec, cp *value.Record, mv *moves) {
	if mv.isFresh(rec) {
		e.copiedFrom(rec, cp)
	}
	owned := e.owned(cp.T)
	if len(owned) == 0 {
		return
	}
	if !mv.isFresh(cp) { // a fresh copy's entries are adopted when its let settles
		e.adoptInto(cp, nil)
	}
	e.rebindFields(cp, &binder{e: e, owned: owned, rt: recordOf(cp.T), owner: cp, inst: e.instance(rec), mode: modeRebind, mv: mv})
}

// adoptInto gives rec's collection fields rec as owner; a fresh entry takes it in place, so the
// refs bound to that entry stay bound to it, and a copy of a fresh collection is fresh.
func (e *Evaluator) adoptInto(rec *value.Record, fresh map[value.Value]bool) {
	rt := recordOf(rec.T)
	if rt == nil {
		return
	}
	for i, f := range fieldsOf(rec.T) {
		c := e.fieldColl(rt, f)
		if c == nil || i >= len(rec.Fields) || rec.Fields[i] == nil {
			continue
		}
		for _, el := range std.Elems(rec.Fields[i]) {
			if en, ok := el.(*value.Record); ok && fresh[en] && en.Ident != nil {
				en.Ident = &value.Identity{Coll: c, Owner: rec, Key: en.Ident.Key, Retired: en.Ident.Retired}
			}
		}
		old := rec.Fields[i]
		if rec.Fields[i] = e.adopt(old, c, rec); fresh[old] && rec.Fields[i] != old {
			fresh[rec.Fields[i]] = true
		}
	}
}

// keyAt is the position of key k in m, -1 when absent; a ref key names an instance (EVALUATION.md §4.2).
func (e *Evaluator) keyAt(m *value.Map, k value.Value) int {
	for i, key := range m.Keys {
		if e.sameKey(key, k) {
			return i
		}
	}
	return -1
}

func (e *Evaluator) sameKey(a, b value.Value) bool {
	x, isRef := a.(*value.Ref)
	y, bothRefs := b.(*value.Ref)
	if !isRef || !bothRefs || x.Owner == nil || y.Owner == nil || x.Key != y.Key {
		return value.Equal(a, b)
	}
	tx, okX := x.T.Base().(*types.RefType)
	ty, okY := y.T.Base().(*types.RefType)
	return okX && okY && tx.Target == ty.Target && e.instance(x.Owner) == e.instance(y.Owner)
}
