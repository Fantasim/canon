package std

import (
	"hash/maphash"
	"math"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// valueSet finds values by value.Equal through a hash equal values share (TYPES.md §7.5).
type valueSet struct {
	seed    maphash.Seed
	buckets map[uint64][]int
	vals    []value.Value
}

func newSet() *valueSet {
	return &valueSet{seed: maphash.MakeSeed(), buckets: map[uint64][]int{}}
}

// index is the position of a value equal to v, -1 when there is none.
func (s *valueSet) index(v value.Value) int {
	for _, i := range s.buckets[s.hash(v)] {
		if value.Equal(s.vals[i], v) {
			return i
		}
	}
	return -1
}

// add adds v unless an equal value is in; it returns v's position and whether it was added.
func (s *valueSet) add(v value.Value) (int, bool) {
	if i := s.index(v); i >= 0 {
		return i, false
	}
	h := s.hash(v)
	s.buckets[h] = append(s.buckets[h], len(s.vals))
	s.vals = append(s.vals, v)
	return len(s.vals) - 1, true
}

func (s *valueSet) hash(v value.Value) uint64 {
	var h maphash.Hash
	h.SetSeed(s.seed)
	writeValue(&h, s.seed, v)
	return h.Sum64()
}

// writeValue hashes v so that equal values hash alike: identities for refs, structure for the
// rest (a record's identity is ignored, as equality ignores it against a plain record), maps
// in any order, -0.0 as 0.0.
func writeValue(h *maphash.Hash, seed maphash.Seed, v value.Value) {
	switch x := v.(type) {
	case *value.Ref:
		if r, ok := x.T.Base().(*types.RefType); ok {
			maphash.WriteComparable(h, identKey{coll: r.Target, owner: x.Owner, key: x.Key})
		}
	case *value.Record:
		maphash.WriteComparable(h, recordDecl(x.T))
		for _, f := range x.Fields {
			if f != nil {
				writeValue(h, seed, f)
			}
		}
	case *value.Map:
		var sum uint64
		for i := range x.Keys {
			var e maphash.Hash
			e.SetSeed(seed)
			writeValue(&e, seed, x.Keys[i])
			writeValue(&e, seed, x.Vals[i])
			sum += e.Sum64()
		}
		maphash.WriteComparable(h, sum)
	default:
		writeScalar(h, seed, v)
	}
}

// writeScalar hashes scalars, lists, tables and pairs.
func writeScalar(h *maphash.Hash, seed maphash.Seed, v value.Value) {
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
	case *value.Pair:
		writeValue(h, seed, x.A)
		writeValue(h, seed, x.B)
	default:
		for _, e := range Elems(v) {
			writeValue(h, seed, e)
		}
	}
}

// recordDecl is the declaration a record value's equality compares: the record of R(args).
func recordDecl(t types.Type) types.Type {
	if a, ok := t.Base().(*types.AppliedRecord); ok {
		return a.Rec
	}
	return t.Base()
}
