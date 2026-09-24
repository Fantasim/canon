package std

import (
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// valueSet finds values, converted to elem first, by a hash equal values share (TYPES.md §7.5).
type valueSet struct {
	host    Host
	elem    types.Type
	buckets map[uint64][]int
	vals    []value.Value
}

func newSet(h Host, elem types.Type) *valueSet {
	return &valueSet{host: h, elem: elem, buckets: map[uint64][]int{}}
}

// index is the position of a value equal to v, -1 when there is none; false: the root aborted.
func (s *valueSet) index(v value.Value) (int, bool) {
	v, ok := s.host.Coerce(v, s.elem)
	if !ok {
		return -1, false
	}
	return s.find(v, value.Hash(v))
}

// add adds v, converted, unless an equal value is in: it returns v's position, whether it was
// added, and false when the root aborted.
func (s *valueSet) add(v value.Value) (int, bool, bool) {
	v, ok := s.host.Coerce(v, s.elem)
	if !ok {
		return -1, false, false
	}
	h := value.Hash(v)
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
