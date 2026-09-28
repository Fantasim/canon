package eval

import (
	"slices"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// moves are the copies one let's amendments made, until it settles: the values they built,
// which no one else holds, and each of those instances replaced by a copy since.
type moves struct {
	fresh   map[value.Value]bool
	lineage map[*value.Record]*value.Record
}

// isFresh reports a value the amendments built, which settle may complete in place.
func (m *moves) isFresh(v value.Value) bool {
	return m != nil && m.fresh[v]
}

// follow records that cp replaces v: fresh when v is, then the latest copy of v's instance.
func (m *moves) follow(v, cp value.Value) {
	if !m.isFresh(v) || v == cp {
		return
	}
	m.fresh[cp] = true
	if old, ok := v.(*value.Record); ok {
		if rec, isRec := cp.(*value.Record); isRec {
			m.lineage[old] = rec
		}
	}
}

// latest is the last copy made of a fresh rec, whose entries may still name it until settled;
// rec itself when none, so no other value reads a copy of it.
func (m *moves) latest(rec *value.Record) *value.Record {
	if m == nil {
		return rec
	}
	for next, ok := m.lineage[rec]; ok; next, ok = m.lineage[rec] {
		rec = next
	}
	return rec
}

// copied is to, the copy an amendment made of from along its path: the same value (carry),
// and a fresh one no one else holds, which settle may complete in place.
func (r *run) copied(from, to value.Value) value.Value {
	r.moving().fresh[to] = true
	if from != nil && to != nil && from != to {
		r.ev.rebuilt[to] = from
	}
	return r.ev.carry(from, to)
}

func (r *run) moving() *moves {
	if r.mv == nil {
		r.mv = &moves{fresh: map[value.Value]bool{}, lineage: map[*value.Record]*value.Record{}}
	}
	return r.mv
}

// moved records that an amendment replaced the instance old by its copy cp (EVALUATION.md §4.1).
func (r *run) moved(old, cp *value.Record) {
	if mv := r.moving(); mv.isFresh(old) {
		r.ev.copiedFrom(old, cp)
		mv.lineage[old] = cp
		return
	}
	r.ev.adoptInto(cp, r.mv.fresh) // old may be another value's: its copy owns its entries at once
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
	if r.mv == nil {
		return
	}
	r.settleIn(v, map[value.Value]bool{})
	r.mv = nil
}

// settleIn settles v's fresh components first, then v, so a copy adoption makes of a fresh
// collection never hides the fresh records below it.
func (r *run) settleIn(v value.Value, done map[value.Value]bool) {
	if !r.mv.isFresh(v) || done[v] {
		return
	}
	done[v] = true
	for _, c := range components(v) {
		r.settleIn(c, done)
	}
	if rec, ok := v.(*value.Record); ok {
		r.ev.adoptInto(rec, r.mv.fresh)
	}
}
