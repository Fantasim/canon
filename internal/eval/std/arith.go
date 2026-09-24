package std

import (
	"math"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// Op is an arithmetic operator of TYPES.md §7.1.
type Op uint8

// Arith is a op b on Int, Float and Duration, with the errors of EVALUATION.md §6.
func Arith(h Host, op Op, a, b value.Value, p *value.Prov) (value.Value, bool) {
	switch x := a.(type) {
	case *value.Int:
		if d, ok := b.(*value.Dur); ok && op == OpMul {
			return durMul(h, d.Ms, x.V, p)
		}
		return intArith(h, op, x.V, intOf(b), p)
	case *value.Float:
		return floatResult(h, floatArith(op, x.V, floatOf(b)), p)
	case *value.Dur:
		return durArith(h, op, x.Ms, b, p)
	}
	return nil, false
}

// add is a + b on numbers (sum's accumulator).
func add(h Host, a, b value.Value, p *value.Prov) (value.Value, bool) {
	return Arith(h, OpAdd, a, b, p)
}

func intOf(v value.Value) int64 {
	if i, ok := v.(*value.Int); ok {
		return i.V
	}
	return 0
}

func floatOf(v value.Value) float64 {
	switch x := v.(type) {
	case *value.Float:
		return x.V
	case *value.Int:
		return float64(x.V)
	}
	return 0
}

func intArith(h Host, op Op, a, b int64, p *value.Prov) (value.Value, bool) {
	if (op == OpDiv || op == OpMod) && b == 0 {
		h.Fail(diag.E4102.At(h.Site()))
		return nil, false
	}
	r, ok := intOps[op](a, b)
	if !ok {
		h.Fail(diag.E4101.AtInteger(h.Site(), h.Site()))
		return nil, false
	}
	return &value.Int{V: r, T: types.IntType, P: p}, true
}

func addInt(a, b int64) (int64, bool) {
	return a + b, (b <= 0 || a <= math.MaxInt64-b) && (b >= 0 || a >= math.MinInt64-b)
}

func subInt(a, b int64) (int64, bool) {
	return a - b, (b >= 0 || a <= math.MaxInt64+b) && (b <= 0 || a >= math.MinInt64+b)
}

func mulInt(a, b int64) (int64, bool) {
	if a == 0 || b == 0 {
		return 0, true
	}
	r := a * b
	return r, r/b == a && (a != -1 || b != math.MinInt64) && (b != -1 || a != math.MinInt64)
}

// divInt truncates toward zero; MIN / -1 overflows (EVALUATION.md §6.1).
func divInt(a, b int64) (int64, bool) {
	return a / b, a != math.MinInt64 || b != -1
}

// modInt has the sign of the left operand; MIN % -1 is 0.
func modInt(a, b int64) (int64, bool) {
	if b == -1 {
		return 0, true
	}
	return a % b, true
}

func floatArith(op Op, a, b float64) float64 {
	switch op {
	case OpAdd:
		return a + b
	case OpSub:
		return a - b
	case OpMul:
		return a * b
	default:
	}
	return a / b
}

// floatResult is E4104 for NaN or an infinity (EVALUATION.md §6.2).
func floatResult(h Host, f float64, p *value.Prov) (value.Value, bool) {
	if math.IsNaN(f) || math.IsInf(f, 0) {
		h.Fail(diag.E4104.At(h.Site(), h.Site()))
		return nil, false
	}
	return &value.Float{V: f, T: types.FloatType, P: p}, true
}

// durArith is the Duration rows of TYPES.md §7.1 (EVALUATION.md §6.3).
func durArith(h Host, op Op, a int64, b value.Value, p *value.Prov) (value.Value, bool) {
	switch y := b.(type) {
	case *value.Dur:
		if op == OpDiv {
			if y.Ms == 0 {
				h.Fail(diag.E4102.At(h.Site()))
				return nil, false
			}
			return floatResult(h, float64(a)/float64(y.Ms), p)
		}
		return durResult(h, intOps[op], a, y.Ms, p)
	case *value.Int:
		if op == OpMul {
			return durMul(h, a, y.V, p)
		}
		if y.V == 0 {
			h.Fail(diag.E4102.At(h.Site()))
			return nil, false
		}
		return durResult(h, divInt, a, y.V, p)
	}
	return nil, false
}

func durMul(h Host, ms, n int64, p *value.Prov) (value.Value, bool) {
	return durResult(h, mulInt, ms, n, p)
}

func durResult(h Host, f func(a, b int64) (int64, bool), a, b int64, p *value.Prov) (value.Value, bool) {
	r, ok := f(a, b)
	if !ok {
		h.Fail(diag.E4101.AtDuration(h.Site(), h.Site()))
		return nil, false
	}
	return &value.Dur{Ms: r, P: p}, true
}

// Neg is unary minus: E4101 on the smallest Int or Duration.
func Neg(h Host, v value.Value, p *value.Prov) (value.Value, bool) {
	switch x := v.(type) {
	case *value.Int:
		return intArith(h, OpSub, 0, x.V, p)
	case *value.Float:
		return &value.Float{V: -x.V, T: types.FloatType, P: p}, true
	case *value.Dur:
		return durResult(h, subInt, 0, x.Ms, p)
	}
	return nil, false
}

// toInt is Int(f), arg the argument of Int, floor, ceil or round (EVALUATION.md §6.2).
func toInt(h Host, f, arg float64, p *value.Prov) (value.Value, bool) {
	if !(f >= minIntFloat && f < maxIntFloat) {
		h.Fail(diag.E4103.At(h.Site(), &value.Float{V: arg, T: types.FloatType}))
		return nil, false
	}
	return &value.Int{V: int64(f), T: types.IntType, P: p}, true
}

// intOps are the checked integer operations, by operator; the bool is false on overflow.
var intOps = [...]func(a, b int64) (int64, bool){
	OpAdd: addInt, OpSub: subInt, OpMul: mulInt, OpDiv: divInt, OpMod: modInt,
}
