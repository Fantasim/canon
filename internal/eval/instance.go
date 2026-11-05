package eval

import (
	"slices"

	"github.com/fantasim/canonlang/internal/eval/std"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

type bindMode uint8

// copiedFrom records that cp is rec's instance when cp's type owns entries, a level-1 target's or
// a collection field's; callers pass only copies no two values diverge in: one sharing rec's
// fields, or one of a value only this let's amendments hold.
func (e *Evaluator) copiedFrom(rec, cp *value.Record) {
	if rec != cp && (len(e.owned(cp.T)) > 0 || e.holdsEntries(cp.T)) {
		e.origin[cp] = rec
	}
}

// holdsEntries reports t a record type with a table or keyed-list field, whose entries name its instance (TYPES.md §6.3).
func (e *Evaluator) holdsEntries(t types.Type) bool {
	rt := recordOf(t)
	if rt == nil {
		return false
	}
	holds, done := e.holding[rt]
	if !done {
		holds = slices.ContainsFunc(fieldsOf(t), func(f *types.Field) bool { return e.fieldColl(rt, f) != nil })
		e.holding[rt] = holds
	}
	return holds
}

// Instance is the earliest record rec copies through copies holding its fields: its instance (EVALUATION.md §4.2).
func (e *Evaluator) Instance(rec *value.Record) *value.Record {
	for next := e.originOf(rec); next != nil && slices.Equal(next.Fields, rec.Fields); next = e.originOf(rec) {
		rec = next
	}
	return rec
}

// instance is the record rec copies, through every copy: the instance rec is. A copy met is then
// linked past the copies holding its fields, never past one that does not, so Instance holds.
func (e *Evaluator) instance(rec *value.Record) *value.Record {
	root := rec
	for next := e.originOf(root); next != nil; next = e.originOf(root) {
		root = next
	}
	for rec != root {
		next := e.originOf(rec)
		if slices.Equal(next.Fields, rec.Fields) {
			e.origin[rec] = e.Instance(next)
		}
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
		e.adoptInto(cp, rec, nil) // no amendment's moves: it never completes cp in place
	}
	e.rebindFields(cp, &binder{e: e, owned: owned, rt: recordOf(cp.T), owner: cp, inst: e.instance(rec), mode: modeRebind, mv: mv})
}

// adoptInto makes rec, a copy of from, own its fields' entries from's instance owns, none does,
// or mv's amendments built, a fresh one in place so the refs bound to it stay bound; another
// instance's entry, or one another place holds, stays its own (DECISIONS 410, 478, 486).
func (e *Evaluator) adoptInto(rec, from *value.Record, mv *moves) {
	rt := recordOf(rec.T)
	if rt == nil {
		return
	}
	holds := &ownerFilter{e: e, inst: e.instance(from), mv: mv}
	for i, f := range fieldsOf(rec.T) {
		c := e.fieldColl(rt, f)
		if c == nil || i >= len(rec.Fields) || rec.Fields[i] == nil {
			continue
		}
		for _, el := range std.Elems(rec.Fields[i]) {
			if en, ok := el.(*value.Record); ok && mv.isFresh(en) && en.Ident != nil {
				en.Ident = &value.Identity{Coll: c, Owner: rec, Key: en.Ident.Key, Retired: en.Ident.Retired}
			}
		}
		old := rec.Fields[i]
		if rec.Fields[i] = e.adoptOf(old, c, rec, holds); mv.isFresh(old) && rec.Fields[i] != old {
			mv.fresh[rec.Fields[i]] = true
		}
	}
}

// ownerFilter holds the entries a copy of inst adopts: inst's, no instance's, those mv's amendments
// built, but none whose owner's instance another place holds (copies.go); nil holds every entry. It
// remembers the last owner met, as a field's entries mostly share one.
type ownerFilter struct {
	e    *Evaluator
	inst *value.Record
	mv   *moves
	last *value.Record
	ok   bool
}

// has reports en, an entry, one of o's.
func (o *ownerFilter) has(en *value.Record) bool {
	if o == nil || en.Ident.Owner == nil || o.mv.made(en) {
		return true
	}
	if owner := en.Ident.Owner; owner != o.last {
		o.last, o.ok = owner, o.e.instance(owner) == o.inst && !o.e.shared(owner, o.mv)
	}
	return o.ok
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
