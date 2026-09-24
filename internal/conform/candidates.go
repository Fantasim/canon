package conform

import (
	"math"
	"slices"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// candidates is parameter i's sorted candidates for a receiver's reads and the fn's test calls, nil for a type ir refuses (CONFORMANCE.md §6.2).
func (s *site) candidates(i int, reads []value.Value, calls []Call) []value.Value {
	args := make([]value.Value, 0, len(calls))
	for _, c := range calls {
		if i < len(c.Args) {
			args = append(args, c.Args[i])
		}
	}
	rng := s.fn.Params[i].Range
	switch t := s.sig.Params[i].Base().(type) {
	case types.Basic:
		if rule := basicRules[t.K]; rule != nil {
			return rule(t, rng, args, reads)
		}
	case *types.EnumType:
		out := make([]value.Value, len(t.Members))
		for m := range t.Members {
			out[m] = &value.Member{Enum: t, Index: m}
		}
		return out
	}
	return nil
}

// candidateRule is the candidates of a scalar parameter of type t.
type candidateRule func(t types.Basic, rng *types.Bound, args, reads []value.Value) []value.Value

// boolCandidates are false and true.
func boolCandidates(types.Basic, *types.Bound, []value.Value, []value.Value) []value.Value {
	return []value.Value{&value.Bool{V: false}, &value.Bool{V: true}}
}

// intCandidates are an integer or Duration parameter's candidates (CONFORMANCE.md §6.2).
func intCandidates(t types.Basic, rng *types.Bound, args, reads []value.Value) []value.Value {
	lo, hi, _ := t.Limits()
	set := slices.Concat(intSeeds[:], []int64{lo, hi})
	for _, a := range args {
		if n, ok := intOf(t.K, a); ok {
			set = append(set, n)
		}
	}
	for _, b := range intBounds(rng) {
		set = around(set, b)
	}
	for _, r := range reads {
		if n, ok := intOf(t.K, r); ok {
			set = around(set, n)
		}
	}
	set = slices.DeleteFunc(set, func(n int64) bool { return n < lo || n > hi })
	slices.Sort(set)
	set = slices.Compact(set)
	out := make([]value.Value, len(set))
	for i, n := range set {
		if t.K == types.Duration {
			out[i] = &value.Dur{Ms: n}
		} else {
			out[i] = &value.Int{V: n, T: t}
		}
	}
	return out
}

// intOf is v as a candidate of an integer (k Int) or Duration parameter: only a value of that kind.
func intOf(k types.Kind, v value.Value) (int64, bool) {
	switch x := v.(type) {
	case *value.Int:
		return x.V, k == types.Int
	case *value.Dur:
		return x.Ms, k == types.Duration
	}
	return 0, false
}

// intBounds are a refinement's finite inclusive bounds: `a..b` ends at b - 1.
func intBounds(rng *types.Bound) []int64 {
	var out []int64
	if rng == nil {
		return out
	}
	if rng.HasLo {
		out = append(out, rng.Lo.I)
	}
	switch {
	case !rng.HasHi:
	case rng.HiIncluded:
		out = append(out, rng.Hi.I)
	case rng.Hi.I > math.MinInt64:
		out = append(out, rng.Hi.I-1)
	}
	return out
}

// around adds n - 1, n and n + 1 to set, leaving out what int64 cannot hold.
func around(set []int64, n int64) []int64 {
	if n > math.MinInt64 {
		set = append(set, n-1)
	}
	set = append(set, n)
	if n < math.MaxInt64 {
		set = append(set, n+1)
	}
	return set
}

// floatCandidates are a Float or Float32 parameter's candidates (CONFORMANCE.md §6.2, DECISIONS 204).
func floatCandidates(t types.Basic, rng *types.Bound, args, reads []value.Value) []value.Value {
	narrow := t.Bits == types.Float32Type.Bits
	hi := math.MaxFloat64
	if narrow {
		hi = math.MaxFloat32
	}
	set := slices.Concat(floatSeeds[:], []float64{math.Copysign(0, -1), -hi, hi})
	for _, a := range args {
		if f, ok := a.(*value.Float); ok {
			set = append(set, f.V)
		}
	}
	for _, b := range floatBounds(rng) {
		set = append(set, adjacent(b, narrow)...)
	}
	for _, r := range reads {
		if f, ok := r.(*value.Float); ok { // CONFORMANCE.md §6.2: v−1, v, v+1 (meta/decisions/log-2026-09-24.md)
			set = append(set, f.V-1, f.V, f.V+1)
		}
	}
	set = slices.DeleteFunc(set, func(f float64) bool { return math.IsNaN(f) || f < -hi || f > hi })
	if narrow {
		for i, f := range set {
			set[i] = float64(float32(f))
		}
	}
	slices.SortFunc(set, compareFloat)
	set = slices.CompactFunc(set, func(a, b float64) bool { return math.Float64bits(a) == math.Float64bits(b) })
	out := make([]value.Value, len(set))
	for i, f := range set {
		out[i] = &value.Float{V: f, T: t}
	}
	return out
}

// floatBounds are a Float refinement's bounds, an exclusive upper one included (DECISIONS 204).
func floatBounds(rng *types.Bound) []float64 {
	var out []float64
	if rng != nil && rng.HasLo {
		out = append(out, rng.Lo.F)
	}
	if rng != nil && rng.HasHi {
		out = append(out, rng.Hi.F)
	}
	return out
}

// adjacent is b's predecessor, b and its successor, binary32-adjacent when narrow (DECISIONS 204; Float32: meta/decisions/log-2026-09-24.md).
func adjacent(b float64, narrow bool) []float64 {
	if narrow {
		f := float32(b)
		down, up := float32(math.Inf(toLower)), float32(math.Inf(toUpper))
		return []float64{float64(math.Nextafter32(f, down)), float64(f), float64(math.Nextafter32(f, up))}
	}
	return []float64{math.Nextafter(b, math.Inf(toLower)), b, math.Nextafter(b, math.Inf(toUpper))}
}

// stringCandidates are a String parameter's: "" and the test arguments, by bytes.
func stringCandidates(t types.Basic, _ *types.Bound, args, _ []value.Value) []value.Value {
	set := []string{""}
	for _, a := range args {
		if s, ok := a.(*value.Str); ok {
			set = append(set, s.V)
		}
	}
	slices.Sort(set)
	set = slices.Compact(set)
	out := make([]value.Value, len(set))
	for i, s := range set {
		out[i] = &value.Str{V: s, T: t}
	}
	return out
}
