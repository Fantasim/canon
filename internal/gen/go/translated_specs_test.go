package gogen_test

import (
	"math"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// guardCount is how many falling-through guards guarded has: a shared tail would make 2^22 copies.
const guardCount = 22

// methods are Potion's translated methods: reads of self through fields, optional paths, a variant and a precomputed method, with the public conversions of CONFORMANCE.md §2.3.
func (c *calc) methods() []spec {
	heal, n := rd(0, intT), prm(0, intT)
	none := &value.None{}
	return []spec{
		{
			name: "healFor", params: params(intT, "missingHp"), result: intT,
			reads: []*ir.Read{{Name: "heal", Path: []string{"heal"}, Type: intT}},
			body:  builtin(ir.BuiltinMin, intT, heal, builtin(ir.BuiltinMax, intT, n, lit(intT, num(0)))),
			vecs:  []vec{ok(vals(num(500)), num(200), num(200)), ok(vals(num(500)), num(500), num(9000)), ok(vals(num(500)), num(0), num(math.MinInt64))},
		},
		{
			name: "withBonus", params: params(intT, "n"), result: intT,
			reads: []*ir.Read{
				{Name: "bonus_value", Path: []string{"bonus", "value"}, Type: intT, Optional: true},
				{Name: "level", Path: []string{"level"}, Type: intT, Optional: true},
			},
			body: bin(ir.OpAdd, intT, &ir.Coalesce{T: intT, X: rd(0, intT), Y: lit(intT, num(0))},
				bin(ir.OpMul, intT, &ir.Coalesce{T: intT, X: rd(1, intT), Y: lit(intT, num(1))}, n)),
			vecs: []vec{
				ok(vals(num(3), num(4)), num(11), num(2)), ok(vals(none, none), num(2), num(2)),
				fails(vals(none, num(5)), eOverflow, num(math.MaxInt64)), ok(vals(num(-1), none), num(-1), num(0)),
			},
		},
		{
			name: "round", params: params(intT, "n"), result: boolT,
			reads: []*ir.Read{{Name: "shape", Path: []string{"shape"}, Type: c.shapeT}},
			body:  bin(ir.OpAnd, boolT, &ir.IsCase{T: boolT, X: rd(0, c.shapeT), Case: 0}, bin(ir.OpGt, boolT, n, lit(intT, num(0)))),
			vecs:  []vec{ok(vals(&value.CaseKind{Index: 0}), yes(true), num(1)), ok(vals(&value.CaseKind{Index: 1}), yes(false), num(1))},
		},
		{
			name: "scaled", params: params(intT, "k"), result: intT,
			reads: []*ir.Read{{Name: "tenth", Path: []string{"tenth"}, Type: intT}},
			body:  bin(ir.OpMul, intT, rd(0, intT), n),
			vecs:  []vec{ok(vals(num(50)), num(150), num(3)), fails(vals(num(50)), eOverflow, num(math.MaxInt64))},
		},
		{
			name: "wait", params: params(intT, "k"), result: durT,
			reads: []*ir.Read{{Name: "cooldown", Path: []string{"cooldown"}, Type: durT}},
			body:  bin(ir.OpMul, durT, rd(0, durT), n),
			vecs: []vec{
				ok(vals(ms(2000)), ms(4000), num(2)), fails(vals(ms(5_000_000_000_000)), eWidth, num(2)),
				fails(vals(ms(2000)), eOverflow, num(math.MaxInt64)),
			},
		},
		{
			name: "heavier", params: params(i32T, "d"), result: i32T,
			reads: []*ir.Read{{Name: "weight", Path: []string{"weight"}, Type: i32T}},
			body:  bin(ir.OpAdd, intT, rd(0, i32T), prm(0, i32T)),
			vecs: []vec{
				ok(vals(num(7)), num(12), num(5)), fails(vals(num(math.MaxInt32)), eWidth, num(1)),
				fails(vals(num(7)), eWidth, num(math.MaxInt32+1)),
			},
		},
		{
			name: "light", params: params(f32T, "f"), result: f32T,
			reads: []*ir.Read{{Name: "ratio", Path: []string{"ratio"}, Type: f32T}},
			body:  bin(ir.OpMul, fltT, rd(0, f32T), prm(0, f32T)),
			vecs: []vec{
				ok(vals(flt(0.5)), flt(1), flt(2)), fails(vals(flt(float64(float32(3e38)))), eF32, flt(2)),
				ok(vals(flt(0.5)), flt(float64(float32(0.1))*0.5), flt(float64(float32(0.1)))),
			},
		},
		{
			name: "label", params: params(intT, "n"), result: strT,
			reads: []*ir.Read{{Name: "name", Path: []string{"name"}, Type: strT}, {Name: "tone", Path: []string{"tone"}, Type: c.toneT}},
			body: &ir.Template{T: strT, Parts: []ir.TemplatePart{
				{X: rd(0, strT)}, {Text: " x", X: n}, {Text: " (", X: rd(1, c.toneT)}, {Text: ")"},
			}},
			vecs: []vec{ok(vals(str("p"), &value.Member{Index: 0}), str("p x3 (soft)"), num(3))},
		},
	}
}

// caseMethod is a method of a case, which reads the case's fields.
func (c *calc) caseMethod() spec {
	return spec{
		name: "scaledR", params: params(intT, "k"), result: intT,
		reads: []*ir.Read{{Name: "r", Path: []string{"r"}, Type: intT}},
		body:  bin(ir.OpMul, intT, rd(0, intT), prm(0, intT)),
		vecs:  []vec{ok(vals(num(2)), num(6), num(3))},
	}
}

// packageFns are public and pure at once: overflow at the int64 limits, division, float
// exactness, rounding, entry and exit checks, evaluation order and short-circuits.
func (c *calc) packageFns() []spec {
	a, b := prm(0, intT), prm(1, intT)
	out := []spec{
		{
			name: "add", params: params(intT, "a", "b"), result: intT, body: bin(ir.OpAdd, intT, a, b),
			vecs: []vec{
				ok(nil, num(3), num(1), num(2)), fails(nil, eOverflow, num(math.MaxInt64), num(1)),
				fails(nil, eOverflow, num(math.MinInt64), num(-1)), ok(nil, num(-1), num(math.MaxInt64), num(math.MinInt64)),
			},
		},
		{
			name: "mul", params: params(intT, "a", "b"), result: intT, body: bin(ir.OpMul, intT, a, b),
			vecs: []vec{
				ok(nil, num(-12), num(-3), num(4)), fails(nil, eOverflow, num(math.MinInt64), num(-1)),
				fails(nil, eOverflow, num(1<<32), num(1<<31)), ok(nil, num(math.MinInt64), num(1<<32), num(-(1 << 31))),
			},
		},
		{
			name: "divmod", params: params(intT, "a", "b"), result: intT,
			body: bin(ir.OpAdd, intT, bin(ir.OpDiv, intT, a, b), bin(ir.OpMod, intT, a, b)),
			vecs: []vec{
				ok(nil, num(4), num(7), num(2)), ok(nil, num(-4), num(-7), num(2)), fails(nil, eDivZero, num(7), num(0)),
				fails(nil, eOverflow, num(math.MinInt64), num(-1)), ok(nil, num(-7), num(7), num(-1)),
			},
		},
		{
			name: "neg", params: params(intT, "a"), result: intT, body: &ir.Unary{T: intT, Op: ir.OpNeg, X: a},
			vecs: []vec{ok(nil, num(-5), num(5)), fails(nil, eOverflow, num(math.MinInt64)), ok(nil, num(-math.MaxInt64), num(math.MaxInt64))},
		},
		{
			name: "absAll", params: params(intT, "a"), result: intT, body: builtin(ir.BuiltinAbs, intT, a),
			vecs: []vec{ok(nil, num(5), num(-5)), fails(nil, eOverflow, num(math.MinInt64))},
		},
		{
			name: "clampTo", params: params(intT, "x", "lo", "hi"), result: intT, body: builtin(ir.BuiltinClamp, intT, a, b, prm(2, intT)),
			vecs: []vec{ok(nil, num(3), num(5), num(0), num(3)), fails(nil, eClamp, num(5), num(3), num(0))},
		},
		{
			name: "order", params: params(intT, "a", "b", "c"), result: intT,
			body: bin(ir.OpAdd, intT, bin(ir.OpMul, intT, a, b), &ir.If{
				T: intT, Cond: bin(ir.OpGt, boolT, prm(2, intT), lit(intT, num(0))),
				Then: prm(2, intT), Else: bin(ir.OpDiv, intT, prm(2, intT), lit(intT, num(0))),
			}),
			vecs: []vec{
				ok(nil, num(7), num(2), num(3), num(1)), fails(nil, eDivZero, num(2), num(3), num(0)),
				fails(nil, eOverflow, num(math.MaxInt64), num(2), num(0)), fails(nil, eOverflow, num(math.MaxInt64), num(2), num(1)),
			},
		},
		{
			// b != 0 and a / b > 1 or a < 0: the division runs only when b is not 0.
			name: "guard", params: params(intT, "a", "b"), result: boolT,
			body: bin(ir.OpOr, boolT, bin(ir.OpAnd, boolT, bin(ir.OpNe, boolT, b, lit(intT, num(0))),
				bin(ir.OpGt, boolT, bin(ir.OpDiv, intT, a, b), lit(intT, num(1)))), bin(ir.OpLt, boolT, a, lit(intT, num(0)))),
			vecs: []vec{
				ok(nil, yes(false), num(4), num(0)), ok(nil, yes(true), num(-4), num(0)), ok(nil, yes(true), num(4), num(2)),
				fails(nil, eOverflow, num(math.MinInt64), num(-1)),
			},
		},
		{
			// c and (if n > 0 { n * 2 } else { n }) > 3: the right side needs statements, run only when c.
			name: "pickUp", params: []*ir.Param{{Name: "c", Type: boolT}, {Name: "n", Type: intT}}, result: boolT,
			body: bin(ir.OpAnd, boolT, prm(0, boolT), bin(ir.OpGt, boolT, &ir.If{
				T: intT, Cond: bin(ir.OpGt, boolT, prm(1, intT), lit(intT, num(0))),
				Then: bin(ir.OpMul, intT, prm(1, intT), lit(intT, num(2))), Else: prm(1, intT),
			}, lit(intT, num(3)))),
			vecs: []vec{
				ok(nil, yes(false), yes(false), num(math.MaxInt64)), fails(nil, eOverflow, yes(true), num(math.MaxInt64)),
				ok(nil, yes(true), yes(true), num(2)), ok(nil, yes(false), yes(true), num(-5)),
			},
		},
	}
	out = append(out, floatFns()...)
	out = append(out, checkedFns()...)
	return append(out, c.stmtFns()...)
}

// quo is a / b in float64 at run time; a constant expression would be exact, then rounded once.
func quo(a, b float64) float64 { return a / b }

// floatFns: IEEE results bit for bit, -0.0, non-finite results, conversions and rounding.
func floatFns() []spec {
	x, y := prm(0, fltT), prm(1, fltT)
	negZero := math.Copysign(0, -1)
	return []spec{
		{
			name: "ratio", params: params(fltT, "a", "b"), result: fltT, body: bin(ir.OpDiv, fltT, x, y),
			vecs: []vec{
				ok(nil, flt(0.25), flt(1), flt(4)), fails(nil, eNotFinite, flt(1), flt(0)),
				ok(nil, flt(negZero), flt(negZero), flt(1)), ok(nil, flt(quo(0.1, 3)), flt(0.1), flt(3)),
			},
		},
		{
			name: "fmin", params: params(fltT, "a", "b", "c"), result: fltT, body: builtin(ir.BuiltinMin, fltT, x, y, prm(2, fltT)),
			vecs: []vec{ok(nil, flt(negZero), flt(0), flt(negZero), flt(1)), ok(nil, flt(negZero), flt(negZero), flt(0), flt(1))},
		},
		{
			name: "rounds", params: params(fltT, "f"), result: intT,
			body: bin(ir.OpAdd, intT, builtin(ir.BuiltinRound, intT, x), builtin(ir.BuiltinInt, intT, x)),
			vecs: []vec{ok(nil, num(5), flt(2.5)), ok(nil, num(-5), flt(-2.5)), fails(nil, eToInt, flt(1e19)), ok(nil, num(-1), flt(-0.5))},
		},
		{
			name: "floors", params: params(fltT, "f"), result: intT,
			body: bin(ir.OpSub, intT, builtin(ir.BuiltinFloor, intT, x), builtin(ir.BuiltinCeil, intT, x)),
			vecs: []vec{ok(nil, num(-1), flt(1.5)), ok(nil, num(0), flt(-2))},
		},
		{
			name: "toF32", params: params(fltT, "x"), result: f32T, body: x,
			vecs: []vec{ok(nil, flt(float64(float32(0.1))), flt(0.1)), fails(nil, eF32, flt(1e39))},
		},
		{
			name: "mixed", params: params(intT, "n"), result: fltT,
			body: bin(ir.OpAdd, fltT, builtin(ir.BuiltinFloat, fltT, prm(0, intT)), builtin(ir.BuiltinAbs, fltT, &ir.Unary{T: fltT, Op: ir.OpNeg, X: lit(fltT, flt(0.5))})),
			vecs: []vec{ok(nil, flt(3.5), num(3))},
		},
		{
			name: "clampF", params: params(fltT, "x", "lo", "hi"), result: fltT, body: builtin(ir.BuiltinClamp, fltT, x, y, prm(2, fltT)),
			vecs: []vec{ok(nil, flt(1), flt(5), flt(0), flt(1)), fails(nil, eClamp, flt(5), flt(1), flt(0))},
		},
	}
}

// checkedFns: parameter checks on entry, result checks on exit, Durations, templates, lookups.
func checkedFns() []spec {
	i8 := prm(0, i8T)
	return []spec{
		{
			name: "grow", params: params(i8T, "x"), result: i8T, body: bin(ir.OpMul, intT, i8, lit(intT, num(2))),
			vecs: []vec{fails(nil, eWidth, num(100)), ok(nil, num(-10), num(-5)), fails(nil, eWidth, num(200))},
		},
		{
			name: "pct", params: params(intT, "x"), result: intT, rng: &types.Bound{HasLo: true, HasHi: true, HiIncluded: true, Hi: types.Limit{I: 100}},
			body: prm(0, intT), vecs: []vec{ok(nil, num(50), num(50)), fails(nil, eRange, num(101)), fails(nil, eRange, num(-1))},
		},
		{
			name: "level", params: []*ir.Param{{Name: "x", Type: intT, Range: &types.Bound{HasLo: true, HasHi: true, Lo: types.Limit{I: 1}, Hi: types.Limit{I: 151}}}},
			result: intT, body: prm(0, intT), vecs: []vec{fails(nil, eRange, num(0)), ok(nil, num(150), num(150)), fails(nil, eRange, num(151))},
		},
		{
			name: "half", params: []*ir.Param{{Name: "f", Type: fltT, Range: &types.Bound{HasLo: true, HasHi: true, Hi: types.Limit{F: 1}}}},
			result: fltT, body: bin(ir.OpDiv, fltT, prm(0, fltT), lit(fltT, flt(2))),
			vecs: []vec{ok(nil, flt(0.25), flt(0.5)), fails(nil, eRange, flt(1)), fails(nil, eRange, flt(-0.5))},
		},
		{
			name: "per", params: params(durT, "a", "b"), result: fltT, body: bin(ir.OpDiv, fltT, prm(0, durT), prm(1, durT)),
			vecs: []vec{ok(nil, flt(4), ms(1000), ms(250)), fails(nil, eDivZero, ms(5), ms(0)), fails(nil, eWidth, ms(9_223_372_036_855), ms(1))},
		},
	}
}

// stmtFns: statement bodies (decision log, ir export fns review): guards falling through, a let scoped to its block and bound again after it, an unused let, an else-if chain; calls of a translated fn and of lookups; parameters named like Go builtins and like the vector's fields.
func (c *calc) stmtFns() []spec {
	x, y := prm(0, intT), prm(1, intT)
	var guards []ir.Stmt
	for i := range int64(guardCount) {
		guards = append(guards, &ir.IfStmt{
			Cond: bin(ir.OpEq, boolT, x, lit(intT, num(i))),
			Then: blk(
				&ir.LetStmt{Name: "z", Value: bin(ir.OpMul, intT, y, lit(intT, num(i+1)))},
				&ir.IfStmt{Cond: bin(ir.OpGt, boolT, lcl("z"), lit(intT, num(i))), Then: blk(ret(lcl("z")))},
			),
		})
	}
	call := func(fn *ir.ExportFn, t ir.TypeRef, args ...ir.PExpr) *ir.CallFn {
		return &ir.CallFn{T: t, Fn: fn, Args: args}
	}
	out := []spec{
		{
			name: "guarded", params: params(intT, "x", "y"), result: intT, body: blk(append(guards, ret(lit(intT, num(0))))...),
			vecs: []vec{
				ok(nil, num(20), num(3), num(5)), ok(nil, num(0), num(30), num(1)), ok(nil, num(0), num(5), num(0)),
				ok(nil, num(22), num(21), num(1)), fails(nil, eOverflow, num(1), num(math.MaxInt64)),
			},
		},
		{
			name: "rescoped", params: params(intT, "x"), result: intT,
			body: blk(
				&ir.IfStmt{Cond: bin(ir.OpGt, boolT, x, lit(intT, num(0))), Then: blk(
					&ir.LetStmt{Name: "z", Value: bin(ir.OpAdd, intT, x, lit(intT, num(1)))},
					&ir.IfStmt{Cond: bin(ir.OpGt, boolT, lcl("z"), lit(intT, num(5))), Then: blk(ret(lcl("z")))},
				)},
				&ir.LetStmt{Name: "z", Value: bin(ir.OpMul, intT, x, lit(intT, num(2)))},
				ret(lcl("z")),
			),
			vecs: []vec{ok(nil, num(8), num(7)), ok(nil, num(4), num(2)), ok(nil, num(-6), num(-3))},
		},
		{
			name: "unused", params: params(intT, "x"), result: intT,
			body: blk(&ir.LetStmt{Name: "z", Value: bin(ir.OpMul, intT, x, lit(intT, num(2)))}, &ir.LetStmt{Name: "one", Value: lit(intT, num(1))}, ret(x)),
			vecs: []vec{ok(nil, num(3), num(3)), fails(nil, eOverflow, num(math.MaxInt64))},
		},
		{
			name: "sign", params: params(intT, "x"), result: intT,
			body: blk(&ir.IfStmt{
				Cond: bin(ir.OpGt, boolT, x, lit(intT, num(0))), Then: blk(ret(lit(intT, num(1)))),
				Else: blk(&ir.IfStmt{Cond: bin(ir.OpLt, boolT, x, lit(intT, num(0))), Then: blk(ret(lit(intT, num(-1)))), Else: blk(ret(lit(intT, num(0))))}),
			}),
			vecs: []vec{ok(nil, num(1), num(5)), ok(nil, num(-1), num(-5)), ok(nil, num(0), num(0))},
		},
		{
			name: "incTwice", params: params(intT, "x"), result: intT, body: call(c.incr, intT, call(c.incr, intT, x)),
			vecs: []vec{ok(nil, num(3), num(1)), fails(nil, eOverflow, num(math.MaxInt64-1))},
		},
		{
			name: "volume", params: []*ir.Param{{Name: "t", Type: c.toneT}, {Name: "n", Type: intT}}, result: intT,
			body: &ir.If{T: intT, Cond: call(c.loud, boolT, prm(0, c.toneT)), Then: y, Else: bin(ir.OpDiv, intT, y, lit(intT, num(2)))},
			vecs: []vec{ok(nil, num(8), &value.Member{Index: 1}, num(8)), ok(nil, num(4), &value.Member{Index: 0}, num(8))},
		},
		{
			name: "sizeFor", params: params(intT, "n"), result: c.sizeRef,
			body: call(c.pick, c.sizeRef, bin(ir.OpGt, boolT, x, lit(intT, num(5)))),
			vecs: []vec{ok(nil, &value.Ref{Key: value.Key{S: "large"}}, num(7)), ok(nil, &value.Ref{Key: value.Key{S: "small"}}, num(1))},
		},
		{
			name: "greet", params: []*ir.Param{{Name: "s", Type: strT}, {Name: "t", Type: c.toneT}}, result: strT,
			body: &ir.Template{T: strT, Parts: []ir.TemplatePart{{Text: "hi ", X: prm(0, strT)}, {Text: " ", X: prm(1, c.toneT)}}},
			vecs: []vec{ok(nil, str(`hi "x" loud`), str(`"x"`), &value.Member{Index: 1})},
		},
		{
			name: "named", params: params(intT, "len", "want", "code", "rt"), result: intT,
			body: bin(ir.OpAdd, intT, bin(ir.OpAdd, intT, x, y), bin(ir.OpAdd, intT, prm(2, intT), prm(3, intT))),
			vecs: []vec{ok(nil, num(10), num(1), num(2), num(3), num(4))},
		},
	}
	return out
}
