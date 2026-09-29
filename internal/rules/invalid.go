package rules

import (
	"github.com/fantasim/canonlang/internal/value"
)

// marks is what an Evaluator may also tell of its invalid marks, which are never removed: with
// it, stage C keeps no answer of its invalid test while no check run marks a value (NFR-02).
type marks interface {
	// InvalidCount is how many values are marked invalid.
	InvalidCount() int
	// InvalidValues is every value marked invalid, in no order.
	InvalidValues() []value.Value
}

// asking answers stage C's invalid test as at each value's first asking (EVALUATION.md §7.3).
type asking struct {
	marks  marks                // nil: every answer is kept from the start
	count  int                  // the marks when the first instance was asked about
	before map[value.Value]bool // the values marked then
	asked  []*value.Record      // the instances asked about while no run marked a value
	below  map[value.Value]bool
}

// ask is whether v or a value under it, refs not followed, is invalid.
func (t *traversal) ask(v value.Value) bool {
	a := &t.asking
	if a.marks == nil {
		return t.kept(v, t.ev.Invalid)
	}
	if a.before == nil {
		a.count, a.before = a.marks.InvalidCount(), markedSet(a.marks)
	}
	if rec, ok := v.(*value.Record); ok {
		a.asked = append(a.asked, rec)
	}
	if a.count == 0 {
		return false // nothing is marked, nor was when any earlier asking was made
	}
	return t.current(v)
}

// kept is the answer for v with invalid telling each value's own mark, kept at its first asking.
func (t *traversal) kept(v value.Value, invalid func(value.Value) bool) bool {
	if v == nil {
		return false
	}
	if known, ok := t.asking.below[v]; ok {
		return known
	}
	bad := invalid(v)
	eachPart(v, nil, func(p part) bool {
		bad = t.kept(p.v, invalid)
		return !bad
	})
	t.asking.below[v] = bad
	return bad
}

// current is the answer for v from the marks as they are, a container's kept.
func (t *traversal) current(v value.Value) bool {
	if v == nil {
		return false
	}
	holds := holdsParts(v)
	if known, ok := t.asking.below[v]; ok && holds {
		return known
	}
	bad := t.ev.Invalid(v)
	if !bad {
		eachPart(v, nil, func(p part) bool {
			bad = t.current(p.v)
			return !bad
		})
	}
	if holds {
		t.asking.below[v] = bad
	}
	return bad
}

// ran notes a check run. Until one marks a value, every answer is the current one and only
// containers' are kept; the first that marks one has the earlier answers rebuilt from the marks
// as they were then, and every answer after is kept.
func (t *traversal) ran() {
	a := &t.asking
	if a.marks == nil || a.before == nil || a.marks.InvalidCount() == a.count {
		return
	}
	was := a.before // the marks every earlier asking was made under
	a.marks, a.below = nil, map[value.Value]bool{}
	for _, rec := range a.asked {
		t.kept(rec, func(v value.Value) bool { return was[v] })
	}
	a.asked, a.before = nil, nil
}

// holdsParts reports a record, list, map, table or pair: a value whose answer is worth keeping.
func holdsParts(v value.Value) bool {
	switch v.(type) {
	case *value.Record, *value.List, *value.Map, *value.Table, *value.Pair:
		return true
	}
	return false
}

// markedSet is the values marks tells are invalid.
func markedSet(m marks) map[value.Value]bool {
	out := map[value.Value]bool{}
	for _, v := range m.InvalidValues() {
		out[v] = true
	}
	return out
}
