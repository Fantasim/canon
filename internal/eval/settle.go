package eval

import (
	"slices"

	"github.com/fantasim/canonlang/internal/eval/std"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// copied is to, the copy an amendment made of from along its path: the same value (carry),
// and a fresh one no one else holds, which settle may complete in place.
func (r *run) copied(from, to value.Value) value.Value {
	if r.fresh == nil {
		r.fresh = map[value.Value]bool{}
	}
	r.fresh[to] = true
	return r.ev.carry(from, to)
}

// moved records that an amendment replaced the instance old by its copy cp.
func (r *run) moved(old, cp *value.Record) {
	if r.lineage == nil {
		r.lineage = map[*value.Record]*value.Record{}
	}
	r.lineage[old] = cp
}

// latest is the last copy the amendments made of rec, rec itself when none.
func latest(lineage map[*value.Record]*value.Record, rec *value.Record) *value.Record {
	for next, ok := lineage[rec]; ok; next, ok = lineage[rec] {
		rec = next
	}
	return rec
}

// boundKey is a path's ref key bound to the nearest amended instance owning its target (EVALUATION.md §9.2).
func (r *run) boundKey(key value.Value, m *amending) value.Value {
	ref, ok := key.(*value.Ref)
	if !ok || ref.Owner != nil {
		return key
	}
	rt, ok := ref.T.Base().(*types.RefType)
	if !ok || rt.Target.Kind != types.CollField {
		return key
	}
	for _, rec := range slices.Backward(m.at) {
		if r.ev.owned(rec.T)[rt.Target] {
			return r.ev.mark(key, &value.Ref{T: ref.T, Key: ref.Key, Owner: rec, P: ref.P})
		}
	}
	return key
}

// settle makes each amended copy own its collections' entries, once per let, walking copies only (TYPES.md §10.2).
func (r *run) settle(v value.Value) {
	if r.fresh == nil {
		return
	}
	r.settleIn(v, map[value.Value]bool{})
	r.fresh, r.lineage = nil, nil
}

// settleIn settles v's fresh components first, then v, so a copy adoption makes of a fresh
// collection never hides the fresh records below it.
func (r *run) settleIn(v value.Value, done map[value.Value]bool) {
	if !r.fresh[v] || done[v] {
		return
	}
	done[v] = true
	for _, c := range components(v) {
		r.settleIn(c, done)
	}
	if rec, ok := v.(*value.Record); ok {
		r.adoptInto(rec)
	}
}

// adoptInto gives rec's collection fields rec as owner; an entry the amendments copied takes
// it in place, so the refs bound to that entry stay bound to it.
func (r *run) adoptInto(rec *value.Record) {
	rt := recordOf(rec.T)
	if rt == nil {
		return
	}
	for i, f := range fieldsOf(rec.T) {
		c := r.ev.fieldColl(rt, f)
		if c == nil || i >= len(rec.Fields) || rec.Fields[i] == nil {
			continue
		}
		for _, el := range std.Elems(rec.Fields[i]) {
			if en, ok := el.(*value.Record); ok && r.fresh[en] && en.Ident != nil {
				en.Ident = &value.Identity{Coll: c, Owner: rec, Key: en.Ident.Key, Retired: en.Ident.Retired}
			}
		}
		rec.Fields[i] = r.ev.adopt(rec.Fields[i], c, rec)
	}
}
