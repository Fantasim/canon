package eval

import (
	"context"
	"slices"

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
// literals recorded, in order, with the root that built each; pending, by value, their entries no walk judged yet.
type keptLits struct {
	v       LiteralVerifier
	recs    []keptRec
	pending map[value.Value][]keptEntry // verdict to come, never copied: a world records its own literals
	next    int
}

// keptRec is a recorded literal, its type, the root whose evaluation built it and its record id.
type keptRec struct {
	st *rootState
	m  *value.Map
	t  *types.DepMapType
	id int
}

// keptEntry is a pending entry of record rec: its value is judged against t at key.
type keptEntry struct {
	rec int
	t   *types.DepMapType
	key value.Value
}

// keepLiteral records m, a literal of kept dependent map type t, for verification; a host that
// verifies no literal, a vector, a test or a capture records none; a world records its own.
func (r *run) keepLiteral(m *value.Map, t *types.DepMapType) {
	e, k := r.ev, &r.ev.lits
	if k.v == nil || r.root == nil || e.parent != nil && e.wld == nil || e.vec != nil || r.test != nil || r.sink != nil {
		return
	}
	r.voidTrace() // a replayed entry would not record it
	k.recs = append(k.recs, keptRec{st: r.root, m: m, t: t, id: k.next})
	for i, v := range m.Vals {
		k.pending[v] = append(k.pending[v], keptEntry{rec: k.next, t: t, key: m.Keys[i]})
	}
	k.next++
}

// JudgedEntry notes v, the entry at key of a dependent map of type t a verification judged
// against T(key): a kept literal holding it there, at an identical type, needs no walk for it.
func (e *Evaluator) JudgedEntry(v value.Value, t types.Type, key value.Value) {
	k := &e.lits
	if len(k.pending) == 0 || k.pending[v] == nil {
		return
	}
	k.take(v, func(x keptEntry) bool { return value.Equal(x.key, key) && sameDepMap(x.t, t) })
}

// sameDepMap reports a and b one dependent map type, as written.
func sameDepMap(a *types.DepMapType, b types.Type) bool {
	return types.Type(a) == b || types.Identical(a, b) && a.String() == b.String()
}

// take removes the pending entries of v that match, reporting whether one did.
func (k *keptLits) take(v value.Value, match func(keptEntry) bool) bool {
	es := k.pending[v]
	left := slices.DeleteFunc(slices.Clone(es), match)
	switch {
	case len(left) == len(es):
		return false
	case len(left) == 0:
		delete(k.pending, v)
	default:
		k.pending[v] = left
	}
	return true
}

// walkLiterals hands each recorded literal of a verified root to the host, its entries still
// pending only, in recording order; false when none was recorded.
func (e *Evaluator) walkLiterals(ctx context.Context) bool {
	recs := e.lits.recs
	if len(recs) == 0 || !e.verifying {
		return false
	}
	e.lits.recs = nil
	for _, rec := range recs {
		left := e.lits.residue(rec)
		if rec.st.status == done && left != nil {
			e.stageB = nil
			e.lits.v.VerifyLiterals(ctx, rec.st.root, []KeptLiteral{{Value: left, Type: rec.t}})
			e.stageB = nil
		}
	}
	return true
}

// residue is rec's literal with only its own entries still pending, which it takes out of pending; nil for none.
func (k *keptLits) residue(rec keptRec) *value.Map {
	var out *value.Map
	for i, v := range rec.m.Vals {
		key := rec.m.Keys[i]
		if !k.take(v, func(x keptEntry) bool { return x.rec == rec.id && x.key == key }) {
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
	k := &e.lits
	kept := k.recs[:0]
	for _, rec := range k.recs {
		if rec.st != st {
			kept = append(kept, rec)
			continue
		}
		for _, v := range rec.m.Vals {
			k.take(v, func(x keptEntry) bool { return x.rec == rec.id })
		}
	}
	k.recs = kept
}
