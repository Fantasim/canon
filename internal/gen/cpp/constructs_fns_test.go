package cppgen_test

import (
	"math"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

var codeFloatToInt = diag.E4103.Def().Code

// fnSpec is a package-level translated fn: parameters, result, body and vectors.
type fnSpec struct {
	name    string
	params  []*ir.Param
	result  ir.TypeRef
	body    ir.PExpr
	vectors [][]value.Value // arguments…, then the result; a code instead of the result is in codes
	codes   []diag.Code
}

func param(i int, t ir.TypeRef) *ir.ParamRef { return &ir.ParamRef{T: t, Index: i} }

func bin(op ir.Op, t ir.TypeRef, x, y ir.PExpr) *ir.Binary {
	return &ir.Binary{T: t, Op: op, X: x, Y: y}
}

func params(t ir.TypeRef, names ...string) []*ir.Param {
	out := make([]*ir.Param, len(names))
	for i, n := range names {
		out[i] = &ir.Param{Name: n, Type: t}
	}
	return out
}

// build turns a spec into the IR of stage E (CONFORMANCE.md §6.5: a result or a code).
func (f fnSpec) build() *ir.ExportFn {
	fn := &ir.ExportFn{Name: f.name, Kind: ir.FnTranslated, Params: f.params, Result: f.result, Body: f.body}
	for i, v := range f.vectors {
		var code diag.Code
		if i < len(f.codes) {
			code = f.codes[i]
		}
		args, want := v[:len(v)-1], v[len(v)-1]
		if code != "" {
			want = nil
		}
		fn.Vectors = append(fn.Vectors, vec(nil, args, want, code))
	}
	return fn
}

func vals(vs ...value.Value) []value.Value { return vs }

// packageFns exercise every helper and the C++ evaluation order (CONFORMANCE.md §2.3, §3).
func packageFns(s *shop) []*ir.ExportFn {
	tone := s.t(s.tone, types.Enum)
	a, b, c, d := param(0, tInt), param(1, tInt), param(2, tInt), param(3, tInt)
	specs := []fnSpec{
		{
			name: "mix", params: params(tInt, "a", "b", "c", "d"), result: tInt,
			body: bin(ir.OpAdd, tInt, bin(ir.OpMul, tInt, a, b), bin(ir.OpDiv, tInt, c, d)),
			vectors: [][]value.Value{
				vals(num(2), num(3), num(10), num(2), num(11)), vals(num(math.MaxInt64), num(2), num(1), num(1), nil),
				vals(num(1), num(1), num(1), num(0), nil), vals(num(math.MaxInt64), num(2), num(1), num(0), nil),
				vals(num(3), num(-4), num(-9), num(2), num(-16)),
			},
			codes: []diag.Code{"", codeOverflow, codeDivZero, codeOverflow},
		},
		{
			name: "label", params: []*ir.Param{{Name: "t", Type: tone}, {Name: "n", Type: tInt}}, result: tString,
			body: &ir.Template{T: tString, Parts: []ir.TemplatePart{{X: param(0, tone)}, {Text: ":", X: b}}},
			vectors: [][]value.Value{
				vals(&value.Member{Index: 0}, num(3), str("warning:3")),
				vals(&value.Member{Index: 2}, num(-2), str("series_1:-2")),
				vals(&value.Member{Index: 3}, num(0), str("legacy:0")),
			},
		},
	}
	specs = append(specs, floatFns()...)
	specs = append(specs, logicFns(s)...)
	specs = append(specs, stmtFns()...)
	specs = append(specs, moreFns(s)...)
	out := make([]*ir.ExportFn, len(specs))
	for i, f := range specs {
		out[i] = f.build()
	}
	return append(out, incTwice(out[len(out)-1]))
}

// incTwice calls the package fn inc: the header then declares the package fns first (log-2026-09-24).
func incTwice(inc *ir.ExportFn) *ir.ExportFn {
	call := func(x ir.PExpr) ir.PExpr { return &ir.CallFn{T: tInt, Fn: inc, Args: []ir.PExpr{x}} }
	f := fnSpec{
		name: "incTwice", params: params(tInt, "x"), result: tInt, body: call(call(param(0, tInt))),
		vectors: [][]value.Value{vals(num(1), num(3)), vals(num(math.MaxInt64-1), nil)},
		codes:   []diag.Code{"", codeOverflow},
	}
	return f.build()
}

// floatFns: Float and Float32 arithmetic, sized results, Durations, short-circuits, lets.
func floatFns() []fnSpec {
	a, b := param(0, tInt), param(1, tInt)
	return []fnSpec{
		{
			name: "ratio", params: params(tFloat, "a", "b"), result: tFloat,
			body: bin(ir.OpDiv, tFloat, param(0, tFloat), param(1, tFloat)),
			vectors: [][]value.Value{
				vals(flt(1), flt(4), flt(0.25)), vals(flt(1), flt(0), nil),
				vals(flt(math.Copysign(0, -1)), flt(1), flt(math.Copysign(0, -1))), vals(flt(0.1), flt(3), flt(0.1/3)),
			},
			codes: []diag.Code{"", codeNotFinite},
		},
		{
			name: "grow", params: params(tInt8, "x"), result: tInt8,
			body:    bin(ir.OpMul, tInt, param(0, tInt8), lit(tInt, num(2))),
			vectors: [][]value.Value{vals(num(100), nil), vals(num(-5), num(-10)), vals(num(200), nil)},
			codes:   []diag.Code{codeWidth, "", codeWidth},
		},
		{
			name: "half", params: params(tDuration, "d"), result: tDuration,
			body:    bin(ir.OpDiv, tDuration, param(0, tDuration), lit(tInt, num(2))),
			vectors: [][]value.Value{vals(dur(3000), dur(1500)), vals(dur(-3), dur(-1))},
		},
		{
			name: "pick", params: []*ir.Param{{Name: "flag", Type: tBool}, {Name: "a", Type: tInt}, {Name: "b", Type: tInt}}, result: tInt,
			body:    &ir.If{T: tInt, Cond: param(0, tBool), Then: param(1, tInt), Else: param(2, tInt)},
			vectors: [][]value.Value{vals(boolean(true), num(1), num(2), num(1)), vals(boolean(false), num(1), num(2), num(2))},
		},
		{
			name: "safe", params: params(tInt, "a", "b"), result: tBool,
			body: bin(ir.OpAnd, tBool, bin(ir.OpNe, tBool, b, lit(tInt, num(0))),
				bin(ir.OpGt, tBool, bin(ir.OpDiv, tInt, a, b), bin(ir.OpMul, tInt, b, b))),
			vectors: [][]value.Value{
				vals(num(4), num(0), boolean(false)), vals(num(20), num(2), boolean(true)),
				vals(num(8), num(2), boolean(false)), vals(num(math.MinInt64), num(-1), nil),
			},
			codes: []diag.Code{"", "", "", codeOverflow},
		},
		{
			name: "around", params: params(tInt, "x"), result: tInt,
			body: &ir.Let{
				T: tInt, Name: "y", Value: bin(ir.OpAdd, tInt, a, lit(tInt, num(1))),
				Body: bin(ir.OpMul, tInt, &ir.LocalRef{T: tInt, Name: "y"}, &ir.LocalRef{T: tInt, Name: "y"}),
			},
			vectors: [][]value.Value{vals(num(3), num(16)), vals(num(math.MaxInt64), nil), vals(num(-1), num(0))},
			codes:   []diag.Code{"", codeOverflow},
		},
	}
}

// logicFns: rounding, clamp, min of three, ordered enums, a range on a parameter.
func logicFns(s *shop) []fnSpec {
	tone := s.t(s.tone, types.Enum)
	a, b, c := param(0, tInt), param(1, tInt), param(2, tInt)
	return []fnSpec{
		{
			name: "shrink", params: params(tFloat, "x"), result: tFloat32,
			body: bin(ir.OpDiv, tFloat, param(0, tFloat), lit(tFloat, flt(3))),
			vectors: [][]value.Value{
				vals(flt(1), flt(float64(float32(1.0/3.0)))), vals(flt(1e300), nil),
				vals(flt(math.Copysign(0, -1)), flt(math.Copysign(0, -1))),
			},
			codes: []diag.Code{"", codeF32},
		},
		{
			name: "rounded", params: params(tFloat, "x"), result: tInt,
			body: &ir.Call{T: tInt, Fn: ir.BuiltinRound, Args: []ir.PExpr{bin(ir.OpMul, tFloat,
				&ir.Call{T: tFloat, Fn: ir.BuiltinAbs, Args: []ir.PExpr{&ir.Unary{T: tFloat, Op: ir.OpNeg, X: param(0, tFloat)}}},
				lit(tFloat, flt(2)))}},
			vectors: [][]value.Value{vals(flt(1.25), num(3)), vals(flt(-1.25), num(3)), vals(flt(1e300), nil)},
			codes:   []diag.Code{"", "", codeFloatToInt},
		},
		{
			name: "clampTo", params: params(tInt, "x", "hi"), result: tInt,
			body:    &ir.Call{T: tInt, Fn: ir.BuiltinClamp, Args: []ir.PExpr{a, lit(tInt, num(0)), b}},
			vectors: [][]value.Value{vals(num(5), num(3), num(3)), vals(num(5), num(-1), nil), vals(num(-5), num(3), num(0))},
			codes:   []diag.Code{"", codeClamp},
		},
		{
			name: "minOf", params: params(tInt, "a", "b", "c"), result: tInt,
			body:    &ir.Call{T: tInt, Fn: ir.BuiltinMin, Args: []ir.PExpr{a, b, c}},
			vectors: [][]value.Value{vals(num(3), num(1), num(2), num(1)), vals(num(-3), num(1), num(2), num(-3))},
		},
		{
			name: "isLoud", params: []*ir.Param{{Name: "t", Type: tone}}, result: tBool,
			body: bin(ir.OpLt, tBool, param(0, tone), lit(tone, &value.Member{Index: 1})),
			vectors: [][]value.Value{
				vals(&value.Member{Index: 0}, boolean(true)), vals(&value.Member{Index: 1}, boolean(false)),
				vals(&value.Member{Index: 2}, boolean(false)),
			},
		},
		{
			name: "levelUp", params: []*ir.Param{{Name: "level", Type: tInt, Range: &types.Bound{Lo: types.Limit{I: 1}, Hi: types.Limit{I: 150}, HasLo: true, HasHi: true}}},
			result: tInt, body: bin(ir.OpAdd, tInt, a, lit(tInt, num(1))),
			vectors: [][]value.Value{vals(num(1), num(2)), vals(num(149), num(150)), vals(num(150), nil), vals(num(0), nil)},
			codes:   []diag.Code{"", "", codeRange, codeRange},
		},
	}
}

// moreFns: Strings, a ref result, an exclusive Float bound, reserved names (CODEGEN.md §3.4).
func moreFns(s *shop) []fnSpec {
	shelfRef := refTo("shelves", s.shelf, true)
	front, back := &value.Ref{Key: value.Key{S: "front"}}, &value.Ref{Key: value.Key{S: "back"}}
	below1 := math.Nextafter(1, 0)
	unit := &types.Bound{Lo: types.Limit{F: 0}, Hi: types.Limit{F: 1}, HasLo: true, HasHi: true}
	return []fnSpec{
		{
			name: "echo", params: params(tString, "s"), result: tString,
			body:    &ir.Let{T: tString, Name: "copy", Value: param(0, tString), Body: &ir.LocalRef{T: tString, Name: "copy"}},
			vectors: [][]value.Value{vals(str("a b"), str("a b")), vals(str(""), str(""))},
		},
		{
			name: "pickShelf", params: params(tBool, "front"), result: shelfRef,
			body:    &ir.If{T: shelfRef, Cond: param(0, tBool), Then: lit(shelfRef, front), Else: lit(shelfRef, back)},
			vectors: [][]value.Value{vals(boolean(true), front), vals(boolean(false), back)},
		},
		{
			name: "unit", params: []*ir.Param{{Name: "x", Type: tFloat, Range: unit}}, result: tFloat, body: param(0, tFloat),
			vectors: [][]value.Value{
				vals(flt(0.5), flt(0.5)), vals(flt(1), nil), vals(flt(below1), flt(below1)), vals(flt(-1), nil),
				vals(flt(math.Copysign(0, -1)), flt(math.Copysign(0, -1))),
			},
			codes: []diag.Code{"", codeRange, "", codeRange},
		},
		{
			name: "inc", params: params(tInt, "new"), result: tInt,
			body: &ir.Let{
				T: tInt, Name: "long", Value: bin(ir.OpAdd, tInt, param(0, tInt), lit(tInt, num(1))),
				Body: &ir.LocalRef{T: tInt, Name: "long"},
			},
			vectors: [][]value.Value{vals(num(1), num(2)), vals(num(math.MaxInt64), nil)},
			codes:   []diag.Code{"", codeOverflow},
		},
	}
}
