package encode

import (
	"math"

	"github.com/fantasim/canonlang/api/vm"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/views/shape"
)

// Bounds are a number type's bounds as the type expressions and controls write them.
type Bounds struct {
	Min, Max                   vm.Number
	MinExclusive, MaxExclusive bool
}

// NumberBounds are the bounds of an integer type, a Float or a Duration (VIEWMODEL.md C12):
// an integer's refinement within its sized limits, a Float's with an exclusive upper bound kept,
// a Duration's refinement in milliseconds.
func NumberBounds(t types.Type) Bounds {
	if t.Base().Kind() == types.Float {
		return floatBounds(t)
	}
	r := IntRange(t)
	var b Bounds
	if r.HasLo {
		b.Min = vm.Int(r.Lo)
	}
	if r.HasHi {
		b.Max = vm.Int(r.Hi)
	}
	return b
}

// Range is the inclusive range of an integer type or a Duration.
type Range struct {
	Lo, Hi       int64
	HasLo, HasHi bool
}

// IntRange is an integer's refinement within its sized limits, a bound absent when it is an
// int64 limit; a Duration's refinement, a bound absent when unwritten.
func IntRange(t types.Type) Range {
	l := shape.LayersOf(t)
	r := Range{Lo: math.MinInt64, Hi: math.MaxInt64}
	if b, ok := t.Base().(types.Basic); ok && b.K == types.Int {
		r.Lo, r.Hi, _ = b.Limits()
	}
	if l.HasLo {
		r.Lo = max(r.Lo, l.Lo.I)
	}
	if l.HasHi {
		r.Hi = min(r.Hi, l.Hi.I)
	}
	r.HasLo, r.HasHi = r.Lo != math.MinInt64, r.Hi != math.MaxInt64
	return r
}

func floatBounds(t types.Type) Bounds {
	l := shape.LayersOf(t)
	bits := types.FloatType.Bits
	if b, ok := t.Base().(types.Basic); ok {
		bits = b.Bits
	}
	var b Bounds
	if l.HasLo {
		b.Min = Float(l.Lo.F, bits)
	}
	if l.HasHi {
		b.Max, b.MaxExclusive = Float(l.Hi.F, bits), !l.HiIncluded
	}
	return b
}

// Lengths are the length bounds of a string, list or map refinement, nil when unwritten.
func Lengths(t types.Type) (lo, hi *int) {
	l := shape.LayersOf(t)
	if l.HasLo {
		n := int(l.Lo.I)
		lo = &n
	}
	if l.HasHi {
		n := int(l.Hi.I)
		hi = &n
	}
	return lo, hi
}

// Pattern is the source of a string's outermost pattern, nil for none (STD-03).
func Pattern(t types.Type) *string {
	l := shape.LayersOf(t)
	if l.Pattern == nil {
		return nil
	}
	s := l.Pattern.String()
	return &s
}
