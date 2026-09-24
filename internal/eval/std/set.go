package std

import (
	"hash/maphash"
	"math"
	"slices"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// valueSet finds values, converted to elem first, by a hash equal values share (TYPES.md §7.5).
type valueSet struct {
	host    Host
	elem    types.Type
	seed    maphash.Seed
	buckets map[uint64][]int
	vals    []value.Value
}

func newSet(h Host, elem types.Type) *valueSet {
	return &valueSet{host: h, elem: elem, seed: maphash.MakeSeed(), buckets: map[uint64][]int{}}
}

// index is the position of a value equal to v, -1 when there is none; false: the root aborted.
func (s *valueSet) index(v value.Value) (int, bool) {
	v, ok := s.host.Coerce(v, s.elem)
	if !ok {
		return -1, false
	}
	return s.find(v, s.hash(v))
}

// add adds v, converted, unless an equal value is in: it returns v's position, whether it was
// added, and false when the root aborted.
func (s *valueSet) add(v value.Value) (int, bool, bool) {
	v, ok := s.host.Coerce(v, s.elem)
	if !ok {
		return -1, false, false
	}
	h := s.hash(v)
	if i, ok := s.find(v, h); !ok || i >= 0 {
		return i, false, ok
	}
	s.buckets[h] = append(s.buckets[h], len(s.vals))
	s.vals = append(s.vals, v)
	return len(s.vals) - 1, true, true
}

// find is the position of a value equal to v among those of hash h, each equality charged
// (DECISIONS 197); false once the root aborted.
func (s *valueSet) find(v value.Value, h uint64) (int, bool) {
	for _, i := range s.buckets[h] {
		eq, ok := s.host.Equal(s.vals[i], v)
		if !ok {
			return -1, false
		}
		if eq {
			return i, true
		}
	}
	return -1, true
}

// hashTask is one step of the hash walk: write v into h or, for sum, write the sum of a map's
// entry hashes into h once they are all written.
type hashTask struct {
	v       value.Value
	h       *maphash.Hash
	entries []*maphash.Hash
	sum     bool
}

// hash walks v on an explicit stack (DECISIONS 195) so that equal values hash alike:
// identities for refs, structure for the rest (a record's identity is ignored, as equality
// ignores it against a plain record), maps in any order, -0.0 as 0.0.
func (s *valueSet) hash(v value.Value) uint64 {
	var h maphash.Hash
	h.SetSeed(s.seed)
	tasks := []hashTask{{v: v, h: &h}}
	for len(tasks) > 0 {
		t := tasks[len(tasks)-1]
		tasks = s.write(t, tasks[:len(tasks)-1])
	}
	return h.Sum64()
}

// write runs task t, pushing the tasks of its components last first, so they run in order.
func (s *valueSet) write(t hashTask, tasks []hashTask) []hashTask {
	if t.sum {
		var sum uint64
		for _, e := range t.entries {
			sum += e.Sum64()
		}
		maphash.WriteComparable(t.h, sum)
		return tasks
	}
	switch x := t.v.(type) {
	case *value.Ref:
		if r, ok := x.T.Base().(*types.RefType); ok {
			maphash.WriteComparable(t.h, identKey{coll: r.Target, owner: x.Owner, key: x.Key})
		}
		return tasks
	case *value.Record:
		maphash.WriteComparable(t.h, recordDecl(x.T))
		return pushAll(tasks, t.h, x.Fields)
	case *value.Map:
		return s.writeMap(x, t.h, tasks)
	case *value.Pair:
		return append(tasks, hashTask{v: x.B, h: t.h}, hashTask{v: x.A, h: t.h})
	}
	if writeScalar(t.h, t.v) {
		return tasks
	}
	return pushAll(tasks, t.h, Elems(t.v))
}

// writeMap hashes each entry apart, then sums them into h, so entry order does not count.
func (s *valueSet) writeMap(m *value.Map, h *maphash.Hash, tasks []hashTask) []hashTask {
	entries := make([]*maphash.Hash, len(m.Keys))
	tasks = append(tasks, hashTask{h: h, entries: entries, sum: true})
	for i := len(m.Keys) - 1; i >= 0; i-- {
		entries[i] = &maphash.Hash{}
		entries[i].SetSeed(s.seed)
		tasks = append(tasks, hashTask{v: m.Vals[i], h: entries[i]}, hashTask{v: m.Keys[i], h: entries[i]})
	}
	return tasks
}

// pushAll pushes a task per value of vs into h, last first; a nil value (an input field) is skipped.
func pushAll(tasks []hashTask, h *maphash.Hash, vs []value.Value) []hashTask {
	for _, v := range slices.Backward(vs) {
		if v != nil {
			tasks = append(tasks, hashTask{v: v, h: h})
		}
	}
	return tasks
}

// writeScalar hashes a scalar; false for any other value.
func writeScalar(h *maphash.Hash, v value.Value) bool {
	switch x := v.(type) {
	case *value.Float:
		f := x.V
		if f == 0 {
			f = 0
		}
		maphash.WriteComparable(h, math.Float64bits(f))
	case *value.Int:
		maphash.WriteComparable(h, x.V)
	case *value.Str:
		maphash.WriteComparable(h, x.V)
	case *value.Dur:
		maphash.WriteComparable(h, x.Ms)
	case *value.Bool:
		maphash.WriteComparable(h, x.V)
	case *value.Member:
		maphash.WriteComparable(h, x.Index)
	case *value.CaseKind:
		maphash.WriteComparable(h, x.Index)
	default:
		return false
	}
	return true
}

// recordDecl is the declaration a record value's equality compares: the record of R(args).
func recordDecl(t types.Type) types.Type {
	if a, ok := t.Base().(*types.AppliedRecord); ok {
		return a.Rec
	}
	return t.Base()
}
