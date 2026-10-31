package gogen

import (
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// The rt helpers of the portable subset by operand kind, Int then Float, Go's native operators and the kinds of a scalar (CONFORMANCE.md §3).
var (
	opHelper    = map[ir.Op]helperPair{ir.OpAdd: {"AddInt", "AddFloat"}, ir.OpSub: {"SubInt", "SubFloat"}, ir.OpMul: {"MulInt", "MulFloat"}, ir.OpDiv: {"DivInt", "DivFloat"}, ir.OpMod: {"ModInt", "ModFloat"}}
	negHelper   = helperPair{"NegInt", "NegFloat"}
	opText      = map[ir.Op]string{ir.OpEq: "==", ir.OpNe: "!=", ir.OpLt: "<", ir.OpLe: "<=", ir.OpGt: ">", ir.OpGe: ">=", ir.OpAnd: "&&", ir.OpOr: "||"}
	scalarKinds = map[types.Kind]bool{types.Bool: true, types.Int: true, types.Float: true, types.String: true, types.Duration: true, types.Enum: true}
	// goImportNames are the packages a generated file may import, which a local never takes (CODEGEN.md §3.4).
	goImportNames = map[string]bool{
		ir.GoRT: true, jsonPkg: true, fmtPkg: true, iterPkg: true, "os": true, atomicName: true, syncPkg: true,
		timePkg: true, strconvPkg: true, stringsPkg: true, slicesPkg: true, mathPkg: true, regexpPkg: true,
		"embed": true, errorsPkg: true, utf8Pkg: true,
	}
)
