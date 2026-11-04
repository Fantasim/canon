package check

import (
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// compoundOps is the operator of each compound assignment (TYPES.md §7.1).
var compoundOps = map[syntax.TokenKind]syntax.TokenKind{
	syntax.TokAddAssign: syntax.TokPlus, syntax.TokSubAssign: syntax.TokMinus,
	syntax.TokMulAssign: syntax.TokStar, syntax.TokDivAssign: syntax.TokSlash,
}

var (
	allArith = map[syntax.TokenKind]bool{syntax.TokPlus: true, syntax.TokMinus: true, syntax.TokStar: true, syntax.TokSlash: true, syntax.TokPercent: true}
	fourOps  = map[syntax.TokenKind]bool{syntax.TokPlus: true, syntax.TokMinus: true, syntax.TokStar: true, syntax.TokSlash: true}
	addSub   = map[syntax.TokenKind]bool{syntax.TokPlus: true, syntax.TokMinus: true}
	mulOnly  = map[syntax.TokenKind]bool{syntax.TokStar: true}
	divOnly  = map[syntax.TokenKind]bool{syntax.TokSlash: true}
	mulDiv   = map[syntax.TokenKind]bool{syntax.TokStar: true, syntax.TokSlash: true}
	plusOnly = map[syntax.TokenKind]bool{syntax.TokPlus: true}
)

var arithRows = []arithRow{
	{ka: types.Int, kb: types.Int, ops: allArith, result: types.IntType},
	{ka: types.Float, kb: types.Float, ops: fourOps, result: types.FloatType},
	{ka: types.Duration, kb: types.Duration, ops: addSub, result: types.DurationType},
	{ka: types.Duration, kb: types.Int, ops: mulDiv, result: types.DurationType},
	{ka: types.Int, kb: types.Duration, ops: mulOnly, result: types.DurationType},
	{ka: types.Duration, kb: types.Duration, ops: divOnly, result: types.FloatType},
	{ka: types.String, kb: types.String, ops: plusOnly, result: types.StringType},
}
