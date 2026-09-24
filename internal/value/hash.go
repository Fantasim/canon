package value

import (
	"hash/maphash"
	"math"
	"slices"
)

// Hash is a hash equal values share, to group them: the first hashNodes nodes of v pre-order,
// each composite with its length; refs by identity, records by structure, a map by its size
// and cached order-free entry sum, -0.0 as 0.0 (DECISIONS 199).
func Hash(v Value) uint64 {
	var h maphash.Hash
	h.SetSeed(hashSeed)
	writeCapped(&h, v, true)
	return h.Sum64()
}

// writeCapped writes the first hashNodes nodes of v into h; a map adds its entry sum when
// sums is set (never inside an entry, so no walk nests).
func writeCapped(h *maphash.Hash, v Value, sums bool) {
	stack := []Value{v}
	for n := 0; len(stack) > 0 && n < hashNodes; n++ {
		x := stack[len(stack)-1]
		stack = writeNode(h, x, stack[:len(stack)-1], hashNodes-n-1, sums)
	}
}

// writeNode writes x itself and pushes, last first, the components of it that the room left
// in the walk can reach.
func writeNode(h *maphash.Hash, x Value, stack []Value, room int, sums bool) []Value {
	switch x := x.(type) {
	case *Ref:
		id := x.identity()
		maphash.WriteComparable(h, identHash{coll: id.Coll, owner: id.Owner, key: id.Key})
	case *Record:
		maphash.WriteComparable(h, decl(x.T))
		maphash.WriteComparable(h, len(x.Fields))
		return pushFirst(stack, x.Fields, room)
	case *List:
		maphash.WriteComparable(h, len(x.Elems))
		return pushFirst(stack, x.Elems, room)
	case *Table:
		maphash.WriteComparable(h, len(x.Entries))
		for _, e := range slices.Backward(x.Entries[:min(room, len(x.Entries))]) {
			stack = append(stack, e)
		}
	case *Map:
		maphash.WriteComparable(h, len(x.Keys))
		if sums {
			maphash.WriteComparable(h, x.entrySum())
		}
	case *Pair:
		maphash.WriteComparable(h, pairLen)
		return append(stack, x.B, x.A)
	default:
		writeScalar(h, x)
	}
	return stack
}

// identHash is what a ref's identity hashes by.
type identHash struct {
	coll, owner any
	key         Key
}

// pushFirst pushes, last first, the first room non-nil values of vs (nil: an input field).
func pushFirst(stack, vs []Value, room int) []Value {
	end, taken := 0, 0
	for end < len(vs) && taken < room {
		if vs[end] != nil {
			taken++
		}
		end++
	}
	for i := end - 1; i >= 0; i-- {
		if vs[i] != nil {
			stack = append(stack, vs[i])
		}
	}
	return stack
}

// writeScalar writes a value that holds no others.
func writeScalar(h *maphash.Hash, v Value) {
	switch x := v.(type) {
	case *Float:
		f := x.V
		if f == 0 {
			f = 0
		}
		maphash.WriteComparable(h, math.Float64bits(f))
	case *Str, *Symbol, *Int, *Dur, *Bool, *Member, *CaseKind:
		maphash.WriteComparable(h, hashKey(x))
	case *Range:
		maphash.WriteComparable(h, x.Start)
		if x.HasEnd {
			maphash.WriteComparable(h, x.End)
		}
	}
}
