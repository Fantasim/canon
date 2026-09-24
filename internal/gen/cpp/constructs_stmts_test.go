package cppgen_test

import (
	"math"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/value"
)

// guardCount is how many falling-through guards guarded has: a shared tail would make 2^22 copies.
const guardCount = 22

func block(stmts ...ir.Stmt) *ir.Block { return &ir.Block{T: tInt, Stmts: stmts} }

func ret(x ir.PExpr) *ir.ReturnStmt { return &ir.ReturnStmt{X: x} }

func local(name string) *ir.LocalRef { return &ir.LocalRef{T: tInt, Name: name} }

// stmtFns: statement bodies (ir.Block), an `if` whose branch falls through to the next statement.
func stmtFns() []fnSpec {
	x, y := param(0, tInt), param(1, tInt)
	var guards []ir.Stmt
	for i := range int64(guardCount) {
		guards = append(guards, &ir.IfStmt{
			Cond: bin(ir.OpEq, tBool, x, lit(tInt, num(i))),
			Then: block(
				&ir.LetStmt{Name: "z", Value: bin(ir.OpMul, tInt, y, lit(tInt, num(i+1)))},
				&ir.IfStmt{Cond: bin(ir.OpGt, tBool, local("z"), lit(tInt, num(i))), Then: block(ret(local("z")))},
			),
		})
	}
	return []fnSpec{
		{
			name: "guarded", params: params(tInt, "x", "y"), result: tInt,
			body: block(append(guards, ret(lit(tInt, num(0))))...),
			vectors: [][]value.Value{
				vals(num(3), num(5), num(20)), vals(num(30), num(1), num(0)), vals(num(5), num(0), num(0)),
				vals(num(21), num(1), num(22)), vals(num(1), num(math.MaxInt64), nil),
			},
			codes: []diag.Code{"", "", "", "", codeOverflow},
		},
		{
			// a branch's `let z` is scoped to its block: the later `let z` is another local.
			name: "rescoped", params: params(tInt, "x"), result: tInt,
			body: block(
				&ir.IfStmt{Cond: bin(ir.OpGt, tBool, x, lit(tInt, num(0))), Then: block(
					&ir.LetStmt{Name: "z", Value: bin(ir.OpAdd, tInt, x, lit(tInt, num(1)))},
					&ir.IfStmt{Cond: bin(ir.OpGt, tBool, local("z"), lit(tInt, num(5))), Then: block(ret(local("z")))},
				), Else: block(ret(bin(ir.OpSub, tInt, lit(tInt, num(0)), x)))},
				&ir.LetStmt{Name: "z", Value: bin(ir.OpMul, tInt, x, lit(tInt, num(2)))},
				ret(local("z")),
			),
			vectors: [][]value.Value{vals(num(7), num(8)), vals(num(2), num(4)), vals(num(-3), num(3))},
		},
	}
}
