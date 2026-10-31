package cppgen

import "github.com/fantasim/canonlang/internal/ir"

// The operators and built-ins of the portable subset (CONFORMANCE.md §3).
var (
	opText = map[ir.Op]string{
		ir.OpAnd: "&&", ir.OpOr: "||", ir.OpEq: "==", ir.OpNe: "!=",
		ir.OpLt: "<", ir.OpLe: "<=", ir.OpGt: ">", ir.OpGe: ">=",
	}
	opHelper = map[ir.Op]string{
		ir.OpAdd: "canon::Add", ir.OpSub: "canon::Sub", ir.OpMul: "canon::Mul", ir.OpDiv: "canon::Div", ir.OpMod: "canon::Mod",
	}
	builtinHelper = map[ir.Builtin]string{
		ir.BuiltinFloat: "canon::IntToFloat", ir.BuiltinInt: "canon::FloatToInt", ir.BuiltinMin: "canon::Min",
		ir.BuiltinMax: "canon::Max", ir.BuiltinAbs: "canon::Abs", ir.BuiltinClamp: "canon::Clamp",
		ir.BuiltinFloor: "canon::FloorFloat", ir.BuiltinCeil: "canon::CeilFloat", ir.BuiltinRound: "canon::RoundFloat",
	}
)
