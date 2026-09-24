package value

import (
	"hash/maphash"
	"math"
	"sync"
)

// keyIndex finds a map's keys by a hash of their value (DECISIONS 199): the positions of the
// keys of each hash for the first n keys of owner, and the sum of its first summed entries' hashes.
type keyIndex struct {
	owner   *Map
	n       int
	buckets map[keyHash][]int
	summed  int
	sum     uint64
}

// indexLocks guard the maps' key indexes, a map taking the lock its pointer hashes to: a
// value may be read from several goroutines.
var indexLocks [indexStripes]sync.Mutex

// stripeSeed and hashSeed seed the lock a map takes and the hashes of Hash; both only group
// values in memory, so no output depends on them.
var (
	stripeSeed = maphash.MakeSeed()
	hashSeed   = maphash.MakeSeed()
)

// keyKind is the kind of a key's hash; values of different kinds are never equal.
type keyKind uint8

// keyHash is a key's hash, equal for equal keys of every key type of TYPES.md §9.2.
type keyHash struct {
	kind  keyKind
	n     int64
	bits  uint64
	s     string
	p     any
	owner *Record
}

// Lookup is the position of key k in v, -1 when it is not a key: only the keys of k's hash
// are compared, with equal, in insertion order; false once equal aborted (DECISIONS 199).
func (v *Map) Lookup(k Value, equal func(a, b Value) (bool, bool)) (int, bool) {
	for _, i := range v.candidates(k) {
		eq, ok := equal(v.Keys[i], k)
		if !ok {
			return -1, false
		}
		if eq {
			return i, true
		}
	}
	return -1, true
}

// candidates are the positions of the keys of k's hash, the index first brought up to date.
func (v *Map) candidates(k Value) []int {
	lock := v.lock()
	lock.Lock()
	defer lock.Unlock()
	ix := v.index()
	for ; ix.n < len(v.Keys); ix.n++ {
		h := hashKey(v.Keys[ix.n])
		ix.buckets[h] = append(ix.buckets[h], ix.n)
	}
	return ix.buckets[hashKey(k)]
}

// entrySum is the order-free sum of the map's entry hashes, each its key's hash and a capped
// hash of its value, computed once per entry and kept with the key index (DECISIONS 199).
func (v *Map) entrySum() uint64 {
	lock := v.lock()
	lock.Lock()
	defer lock.Unlock()
	ix := v.index()
	for ; ix.summed < len(v.Keys); ix.summed++ {
		var h maphash.Hash
		h.SetSeed(hashSeed)
		maphash.WriteComparable(&h, hashKey(v.Keys[ix.summed]))
		writeCapped(&h, v.Vals[ix.summed], false)
		ix.sum += h.Sum64()
	}
	return ix.sum
}

// lock is the lock guarding v's index.
func (v *Map) lock() *sync.Mutex {
	return &indexLocks[maphash.Comparable(stripeSeed, v)%indexStripes]
}

// index is v's key index, a new one when it has none, or holds a copied map's or more keys
// than v has; the caller holds v's lock.
func (v *Map) index() *keyIndex {
	if v.idx == nil || v.idx.owner != v || v.idx.n > len(v.Keys) || v.idx.summed > len(v.Keys) {
		v.idx = &keyIndex{owner: v, buckets: map[keyHash][]int{}}
	}
	return v.idx
}

// hashKey is the hash of a key: its identity for a ref or an entry, else its scalar value.
func hashKey(v Value) keyHash {
	if id := identity(v); id != nil {
		kind := keyIdent
		if id.Key.IsInt {
			kind = keyIdentInt
		}
		return keyHash{kind: kind, n: id.Key.I, s: id.Key.S, p: id.Coll, owner: id.Owner}
	}
	switch x := v.(type) {
	case *Int:
		return keyHash{kind: keyInt, n: x.V}
	case *Str:
		return keyHash{kind: keyStr, s: x.V}
	case *Member:
		return keyHash{kind: keyMember, n: int64(x.Index), p: x.Enum}
	case *CaseKind:
		return keyHash{kind: keyCase, n: int64(x.Index), p: x.T.Variant}
	case *Symbol:
		return keyHash{kind: keySymbol, s: x.Name}
	case *Dur:
		return keyHash{kind: keyDur, n: x.Ms}
	case *Bool:
		if x.V {
			return keyHash{kind: keyBool, n: 1}
		}
		return keyHash{kind: keyBool}
	case *Float:
		f := x.V
		if f == 0 {
			f = 0
		}
		return keyHash{kind: keyFloat, bits: math.Float64bits(f)}
	}
	return keyHash{kind: keyOther}
}
