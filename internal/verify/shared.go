package verify

import (
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// sharedKey is a container and the type it is walked against. A walk counts in spent the dependent
// judgements and type arguments it reads, and in effects its findings, marks, uses and reads.
type sharedKey struct {
	v value.Value
	t types.Type
}

// sharedVisit is a container's first visit that spent nothing: whether it was a past slot, and what it found, nil for nothing.
type sharedVisit struct {
	past  bool
	found *sharedFound
}

// sharedFound is where a visit that found something ran: its entries and its innermost instance being recorded.
type sharedFound struct {
	entry, retired string
	top            *recorder
}

// visit verifies v at at once per walk when it is a container: a later visit is skipped when the
// first spent nothing and would find nothing new, its findings kept at the first place (DECISIONS 509).
func (w *walker) visit(v value.Value, t types.Type, at *Path, sc scope) value.Value {
	switch v.(type) {
	case *value.Record, *value.List, *value.Map, *value.Pair:
	default:
		return w.verified(v, t, at, sc)
	}
	k := sharedKey{v: v, t: t}
	if sv, ok := w.shared[k]; ok && w.covers(sv, sc) {
		return v
	}
	spent, effects := w.spent, w.effects
	nv := w.verified(v, t, at, sc)
	if nv != v || w.spent != spent || w.stopped() {
		return nv
	}
	sv := sharedVisit{past: sc.past}
	if w.effects != effects {
		sv.found = &sharedFound{entry: sc.entry, retired: sc.retired, top: w.top()}
	}
	if w.shared == nil {
		w.shared = map[sharedKey]sharedVisit{}
	}
	w.shared[k] = sv
	return nv
}

// covers reports sv's visit did all a visit in sc would: one that found nothing, in a slot no more
// hidden than sc's; one that found something, in sc's own scope, under instances it was recorded for.
func (w *walker) covers(sv sharedVisit, sc scope) bool {
	f := sv.found
	if f == nil {
		return !sv.past || sc.past
	}
	// Under a stage each table entry's instance pushes a recorder, so the recorder check implies the entry one; a fixture has no stage.
	if sv.past != sc.past || f.entry != sc.entry || f.retired != sc.retired {
		return false
	}
	top := w.top()
	for r := f.top; top != nil; r = r.parent {
		if r == top {
			return true
		}
		if r == nil {
			return false
		}
	}
	return true
}

// top is the innermost instance being recorded, nil for none.
func (w *walker) top() *recorder {
	if len(w.recording) == 0 {
		return nil
	}
	return w.recording[len(w.recording)-1]
}

// push records rec inside the instances being recorded.
func (w *walker) push(rec *recorder) {
	rec.parent = w.top()
	w.recording = append(w.recording, rec)
}

// apart gives a top-level collection's entry a memo of its own, so what it keeps is freed when the
// entry is verified; it hands back the memo around it.
func (w *walker) apart() map[sharedKey]sharedVisit {
	outer := w.shared
	w.shared, w.spare = w.spare, nil
	return outer
}

// together puts back the memo around an entry, keeping the entry's, emptied, for the next.
func (w *walker) together(outer map[sharedKey]sharedVisit) {
	clear(w.shared)
	w.spare, w.shared = w.shared, outer
}
