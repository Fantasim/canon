package eval

import (
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// keepRecord records rec, a record or case literal with a dependent field, where r's literals go, as a kept map literal (DECISIONS 446).
func (r *run) keepRecord(rec *value.Record) {
	k := &r.ev.lits
	sink, st := r.literalSink()
	if sink == nil || !r.ev.judgedApart(rec) {
		return
	}
	r.traceRecord(rec, sink, st)
	kr := keptRec{st: st, r: rec, id: k.next, sub: r.sub, free: r.free}
	sink.recs = append(sink.recs, kr)
	sink.pending.add(kr)
	k.stands(rec, rec)
	k.next++
}

// judgedApart reports rec judged as a value: its type has a dependent field, an applied one its own arguments (TYPES.md §11.1, §11.4).
func (e *Evaluator) judgedApart(rec *value.Record) bool {
	if _, applied := rec.T.Base().(*types.AppliedRecord); applied && e.boundParams(rec) == nil {
		return false
	}
	return e.depFields(rec.T)
}

// depFields reports a record or case type with a field whose type holds a dependent type, memoised by type.
func (e *Evaluator) depFields(t types.Type) bool {
	k := &e.lits
	if d, ok := k.depRecs[t]; ok {
		return d
	}
	d := false
	for _, f := range fieldsOf(t) {
		d = d || e.dependent(f.Type)
	}
	if k.depRecs == nil {
		k.depRecs = map[types.Type]bool{}
	}
	k.depRecs[t] = d
	return d
}

// stands makes rec, a record literal or a copy of one, stand for the literal lit.
func (k *keptLits) stands(rec, lit *value.Record) {
	if k.literal == nil {
		k.literal = map[*value.Record]*value.Record{}
	}
	k.literal[rec] = lit
}

// copied makes to, a copy of from, stand for the record literal from stands for, if any, which a walk of to judges (DECISIONS 446).
func (k *keptLits) copied(from, to value.Value) {
	src, isRec := from.(*value.Record)
	if !isRec || k.literal[src] == nil {
		return
	}
	if cp, ok := to.(*value.Record); ok {
		k.stands(cp, k.literal[src])
	}
}

// JudgedRecord notes rec, a record a verification walked at the type of the slot holding it: the record literal it stands for needs no walk of its own (DECISIONS 446).
func (e *Evaluator) JudgedRecord(rec *value.Record) {
	k := &e.lits
	lit := k.literal[rec]
	if lit == nil {
		return
	}
	if k.judging == nil {
		k.roots.pending.judgedRecord(lit)
		return
	}
	for s := k.judging; s != nil; s = s.up {
		s.pending.judgedRecord(lit)
	}
}

// judgedRecord takes record literal lit out of p.
func (p pendingSet) judgedRecord(lit *value.Record) {
	if len(p) == 0 || p[lit] == nil {
		return
	}
	p.take(lit, func(x keptEntry) bool { return x.t == nil })
}

// adoptRecord makes the copy of rec, a record literal of s a substituted value holds, its let's (EVALUATION.md §10.5); false: none held.
func (w *World) adoptRecord(s *litScope, rec keptRec, copies map[value.Value]value.Value) bool {
	cr, held := copies[rec.r].(*value.Record)
	if !held || rec.sub == nil {
		return false
	}
	k := &w.ev.lits
	wr := keptRec{st: w.ev.state(rec.sub), r: cr, id: k.next}
	k.next++
	if s.pending.take(rec.r, func(x keptEntry) bool { return x.rec == rec.id }) {
		k.roots.pending.add(wr)
		k.stands(cr, cr)
	}
	k.roots.recs = append(k.roots.recs, wr)
	return true
}

// resultHolds takes out of s the record literals v, a precomputed result its caller verifies, holds.
func (k *keptLits) resultHolds(s *litScope, v value.Value) {
	if s == nil || len(s.recs) == 0 {
		return
	}
	held := k.heldLiterals(v)
	for _, rec := range s.recs {
		if rec.r != nil && held[rec.r] {
			s.pending.take(rec.r, func(x keptEntry) bool { return x.rec == rec.id && x.t == nil })
		}
	}
}

// heldLiterals is the record literals each record of v stands for, v's parts walked once each.
func (k *keptLits) heldLiterals(v value.Value) map[*value.Record]bool {
	held := map[*value.Record]bool{}
	seen := map[value.Value]bool{}
	stack := []value.Value{v}
	for len(stack) > 0 {
		x := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if x == nil || seen[x] {
			continue
		}
		seen[x] = true
		if rec, ok := x.(*value.Record); ok && k.literal[rec] != nil {
			held[k.literal[rec]] = true
		}
		stack = append(stack, components(x)...)
	}
	return held
}
