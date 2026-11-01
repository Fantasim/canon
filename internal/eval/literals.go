package eval

import (
	"context"
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// LiteralVerifier is a Host that also verifies the kept dependent map literals a top-level value's evaluation built, stored or not (TYPES.md §11.5).
type LiteralVerifier interface {
	// VerifyLiterals runs stage B on each literal, its steps and findings root's, once the values of a flush are verified (EVALUATION.md §5).
	VerifyLiterals(ctx context.Context, root Root, lits []KeptLiteral)
}

// KeptLiteral is a map literal typed against a kept dependent map, its entries no root's verification judged.
type KeptLiteral struct {
	Value value.Value
	Type  types.Type
}

// keptLits are the kept literals the evaluator verifies: the host verifying them, nil for none; the
// roots' literals, in order, each with the root that built it, and their entries no walk judged yet.
type keptLits struct {
	v       LiteralVerifier
	roots   litScope // verdict to come, never copied: a world records its own literals
	next    int
	judging *litScope // the scope whose entries a verification judges now; nil: the roots'
	free    bool      // the literal being judged was built by a where re-run, which costs nothing (DECISIONS 148)
}

// pendingSet is, by value, the entries of recorded literals no walk judged yet.
type pendingSet map[value.Value][]keptEntry

// keptRec is a recorded literal, its type, the root whose evaluation built it and its record id;
// sub, in a substitution's scope, is the let it is evaluated for; free, a where re-run built it.
type keptRec struct {
	st   *rootState
	m    *value.Map
	t    *types.DepMapType
	id   int
	sub  check.Object
	free bool
}

// keptEntry is a pending entry of record rec: its value is judged against t at key.
type keptEntry struct {
	rec int
	t   *types.DepMapType
	key value.Value
}

// keepLiteral records m, a literal of kept dependent map type t, for verification where r's
// literals go (literalSink); none when they go nowhere.
func (r *run) keepLiteral(m *value.Map, t *types.DepMapType) {
	k := &r.ev.lits
	sink, st := r.literalSink()
	if sink == nil {
		return
	}
	r.voidTrace() // a replayed entry would not record it
	// a where only stage B runs is free, and so is its literal (DECISIONS 148; log-2026-11-01 "EVAL-OWED review FAIL")
	rec := keptRec{st: st, m: m, t: t, id: k.next, sub: r.sub, free: r.free}
	sink.recs = append(sink.recs, rec)
	sink.pending.add(rec)
	k.next++
}

// literalSink is where r records a kept literal, and the root it is charged to: a root's run's
// root, the scope of a run evaluating none, or the value a verification run serves; nil: nowhere.
func (r *run) literalSink() (*litScope, *rootState) {
	k := &r.ev.lits
	switch {
	case r.root != nil:
		if k.v != nil && r.test == nil && r.sink == nil {
			return &k.roots, r.root
		}
	case r.lits != nil:
		return r.lits, nil
	case r.stage != nil:
		return r.ev.stageSink(r.stage.root)
	}
	return nil, nil
}

// stageSink is where root's verification records a literal: the scope judged for, else root's (EVALUATION.md §5).
func (e *Evaluator) stageSink(root Root) (*litScope, *rootState) {
	k := &e.lits
	if k.judging != nil {
		return k.judging, nil
	}
	if st := e.rootState(root); st != nil && k.v != nil {
		return &k.roots, st
	}
	return nil, nil
}

// add makes rec's entries pending.
func (p pendingSet) add(rec keptRec) {
	for i, v := range rec.m.Vals {
		p[v] = append(p[v], keptEntry{rec: rec.id, t: rec.t, key: rec.m.Keys[i]})
	}
}

// JudgedEntry notes v, the entry at key of a dependent map of type t a verification judged
// against T(key): a kept literal holding it there, at an identical type, needs no walk for it.
func (e *Evaluator) JudgedEntry(v value.Value, t types.Type, key value.Value) {
	k := &e.lits
	if k.judging == nil {
		k.roots.pending.judged(v, t, key)
		return
	}
	for s := k.judging; s != nil; s = s.up {
		s.pending.judged(v, t, key)
	}
}

// judged takes out of p the entries of v at key whose type is t.
func (p pendingSet) judged(v value.Value, t types.Type, key value.Value) {
	if len(p) == 0 || p[v] == nil {
		return
	}
	p.take(v, func(x keptEntry) bool { return value.Equal(x.key, key) && sameDepMap(x.t, t) })
}

// sameDepMap reports a and b one dependent map type, as written.
func sameDepMap(a *types.DepMapType, b types.Type) bool {
	return types.Type(a) == b || types.Identical(a, b) && a.String() == b.String()
}

// take removes the pending entries of v that match, reporting whether one did.
func (p pendingSet) take(v value.Value, match func(keptEntry) bool) bool {
	es := p[v]
	left := slices.DeleteFunc(slices.Clone(es), match)
	switch {
	case len(left) == len(es):
		return false
	case len(left) == 0:
		delete(p, v)
	default:
		p[v] = left
	}
	return true
}

// walkLiterals hands each recorded literal of a verified root to the host, its entries still
// pending only, in recording order; false when none was recorded.
func (e *Evaluator) walkLiterals(ctx context.Context) bool {
	k := &e.lits
	recs := k.roots.recs
	if len(recs) == 0 || !e.verifying {
		return false
	}
	k.roots.recs = nil
	defer e.judgingIn(nil)()
	for _, rec := range recs {
		left := k.roots.pending.residue(rec)
		if rec.st.status == done && left != nil {
			e.judgeOne(rec, left, func(l KeptLiteral) bool {
				k.v.VerifyLiterals(ctx, rec.st.root, []KeptLiteral{l})
				return true
			})
		}
	}
	return true
}

// judgeOne hands rec's residue left to judge, free when a where re-run built it.
func (e *Evaluator) judgeOne(rec keptRec, left *value.Map, judge func(KeptLiteral) bool) bool {
	free := e.lits.free
	e.stageB, e.lits.free = nil, rec.free
	defer func() { e.stageB, e.lits.free = nil, free }()
	return judge(KeptLiteral{Value: left, Type: rec.t})
}

// residue is rec's literal with only its own entries still pending, which it takes out of p; nil for none.
func (p pendingSet) residue(rec keptRec) *value.Map {
	var out *value.Map
	for i, v := range rec.m.Vals {
		key := rec.m.Keys[i]
		if !p.take(v, func(x keptEntry) bool { return x.rec == rec.id && x.key == key }) {
			continue
		}
		if out == nil {
			out = &value.Map{T: rec.m.T, P: rec.m.P}
		}
		out.Keys, out.Vals = append(out.Keys, key), append(out.Vals, v)
	}
	return out
}

// dropLiterals forgets the literals st's evaluation recorded, and only their entries: it was poisoned or cancelled.
func (e *Evaluator) dropLiterals(st *rootState) {
	s := &e.lits.roots
	kept := s.recs[:0]
	for _, rec := range s.recs {
		if rec.st != st {
			kept = append(kept, rec)
			continue
		}
		for _, v := range rec.m.Vals {
			s.pending.take(v, func(x keptEntry) bool { return x.rec == rec.id })
		}
	}
	s.recs = kept
}
