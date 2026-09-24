package std

import (
	"math"

	"github.com/fantasim/canonlang/internal/value"
)

// Less is a < b on orderable values (TYPES.md §7.5).
func Less(a, b value.Value) bool {
	switch x := a.(type) {
	case *value.Int:
		if y, ok := b.(*value.Int); ok {
			return x.V < y.V
		}
		return float64(x.V) < floatOf(b)
	case *value.Float:
		return x.V < floatOf(b)
	case *value.Dur:
		y, ok := b.(*value.Dur)
		return ok && x.Ms < y.Ms
	case *value.Str:
		y, ok := b.(*value.Str)
		return ok && x.V < y.V
	case *value.Member:
		y, ok := b.(*value.Member)
		return ok && x.Index < y.Index
	}
	return false
}

// before is the order of min and max: Less, with -0.0 before +0.0 (STDLIB.md §2.2).
func before(a, b value.Value) bool {
	x, okA := a.(*value.Float)
	y, okB := b.(*value.Float)
	if okA && okB && x.V == y.V {
		return math.Signbit(x.V) && !math.Signbit(y.V)
	}
	return Less(a, b)
}
