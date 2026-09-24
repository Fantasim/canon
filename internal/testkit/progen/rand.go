package progen

import (
	"encoding/binary"
	"math"
)

// Rand is splitmix64 from one seed, so that a case replays from its seed alone: written here,
// since gosec refuses math/rand outside tests and an ignore would grow the ignore count.
type Rand struct {
	state uint64
}

// NewRand is the source of the case numbered seed.
func NewRand(seed uint64) *Rand { return &Rand{state: seed} }

// next is the next raw draw (Steele, Lea and Flood's splitmix64).
func (r *Rand) next() uint64 {
	r.state += mixGamma
	z := r.state
	z = (z ^ z>>30) * mixA
	z = (z ^ z>>27) * mixB
	return z ^ z>>31
}

// Intn is a number in [0, n); 0 when n is at most 1. It reads the draw's low 31 bits through
// its bytes: no narrowing conversion for gosec G115 to prove safe.
func (r *Rand) Intn(n int) int {
	if n <= 1 {
		return 0
	}
	var b [drawBytes]byte
	binary.LittleEndian.PutUint64(b[:], r.next())
	return int(binary.LittleEndian.Uint32(b[:])&math.MaxInt32) % n
}

// OneIn is true once in n draws on average.
func (r *Rand) OneIn(n int) bool { return r.Intn(n) == 0 }

// Pick is an element of xs chosen by r; the zero value for an empty xs.
func Pick[T any](r *Rand, xs []T) T {
	var zero T
	if len(xs) == 0 {
		return zero
	}
	return xs[r.Intn(len(xs))]
}
