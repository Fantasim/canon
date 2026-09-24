package std

import (
	"math"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// freeFunctions are STDLIB.md §2.
var freeFunctions = map[string]builtin{
	bInt: convInt, bFloat: convFloat, bString: convString, bAbs: mathAbs, bMin: mathMin,
	bMax: mathMax, bClamp: mathClamp, bFloor: mathFloor, bCeil: mathCeil, bRound: mathRound,
	bSqrt: mathSqrt, bPow: mathPow, bReachable: graphReachable, bCycles: graphCycles,
	bTopoSort: graphTopoSort,
}

// convInt is Int(x): a Float truncated (E4103), an integer of any width as Int.
func convInt(h Host, c *Call) (value.Value, bool) {
	switch x := c.arg(0).(type) {
	case *value.Float:
		return toInt(h, x.V, x.V, c.Prov)
	case *value.Int:
		return &value.Int{V: x.V, T: types.IntType, P: c.Prov}, true
	}
	return nil, false
}

// convFloat is Float(x): exact up to 2^53, else nearest-even, as Go's conversion rounds.
func convFloat(_ Host, c *Call) (value.Value, bool) {
	return &value.Float{V: floatOf(c.arg(0)), T: types.FloatType, P: c.Prov}, true
}

// convString is String(x): values visited, then bytes, charged before the text (STDLIB.md §9.1).
func convString(h Host, c *Call) (value.Value, bool) {
	x := c.arg(0)
	if !h.Charge(VisitedUpTo(x, h.Remaining())) || !h.Charge(value.TextLenUpTo(x, h.Remaining())) {
		return nil, false
	}
	return c.strv(x.CanonText()), true
}

// VisitedUpTo is the number of values the text form of v visits (1 for a scalar, 1 plus its
// components for a composite), counted on an explicit stack and stopping at limit (DECISIONS 195).
func VisitedUpTo(v value.Value, limit int) int {
	n := 0
	stack := []value.Value{v}
	for len(stack) > 0 && n < limit {
		x := stack[len(stack)-1]
		stack = append(stack[:len(stack)-1], parts(x)...)
		n++
	}
	return n
}

// parts are the components of a composite value, in order: fields (inputs skipped),
// elements, entries, map keys and values alternately, a pair's halves.
func parts(v value.Value) []value.Value {
	var out []value.Value
	switch x := v.(type) {
	case *value.Record:
		for _, f := range x.Fields {
			if f != nil {
				out = append(out, f)
			}
		}
	case *value.Map:
		for i := range x.Keys {
			out = append(out, x.Keys[i], x.Vals[i])
		}
	case *value.Pair:
		out = append(out, x.A, x.B)
	default:
		out = Elems(v)
	}
	return out
}

// mathAbs is E4101 for the smallest Int or Duration.
func mathAbs(h Host, c *Call) (value.Value, bool) {
	x := c.arg(0)
	if Less(x, zeroOf(x.Type(), nil)) {
		return Neg(h, x, c.Prov)
	}
	if f, ok := x.(*value.Float); ok {
		return &value.Float{V: math.Abs(f.V), T: types.FloatType, P: c.Prov}, true
	}
	return x, true
}

// mathMin and mathMax return the first of equal arguments (STDLIB.md §2.2).
func mathMin(_ Host, c *Call) (value.Value, bool) {
	return pick(c.Args, before), true
}

func mathMax(_ Host, c *Call) (value.Value, bool) {
	return pick(c.Args, func(a, b value.Value) bool { return before(b, a) }), true
}

func pick(xs []value.Value, better func(a, b value.Value) bool) value.Value {
	best := xs[0]
	for _, x := range xs[1:] {
		if better(x, best) {
			best = x
		}
	}
	return best
}

// mathClamp is min(max(x, lo), hi); E4108 when lo > hi.
func mathClamp(h Host, c *Call) (value.Value, bool) {
	x, lo, hi := c.arg(0), c.arg(argLo), c.arg(argHi)
	if Less(hi, lo) {
		h.Fail(diag.E4108.At(h.Site(), lo, hi))
		return nil, false
	}
	low := pick([]value.Value{x, lo}, func(a, b value.Value) bool { return before(b, a) })
	return pick([]value.Value{low, hi}, before), true
}

func mathFloor(h Host, c *Call) (value.Value, bool) {
	x := floatOf(c.arg(0))
	return toInt(h, math.Floor(x), x, c.Prov)
}

func mathCeil(h Host, c *Call) (value.Value, bool) {
	x := floatOf(c.arg(0))
	return toInt(h, math.Ceil(x), x, c.Prov)
}

// mathRound rounds half away from zero.
func mathRound(h Host, c *Call) (value.Value, bool) {
	x := floatOf(c.arg(0))
	return toInt(h, math.Round(x), x, c.Prov)
}

// mathSqrt is E4104 for a negative argument.
func mathSqrt(h Host, c *Call) (value.Value, bool) {
	return floatResult(h, math.Sqrt(floatOf(c.arg(0))), c.Prov)
}

// mathPow is E4104 when the result is NaN or infinite.
func mathPow(h Host, c *Call) (value.Value, bool) {
	return floatResult(h, math.Pow(floatOf(c.arg(0)), floatOf(c.arg(1))), c.Prov)
}
