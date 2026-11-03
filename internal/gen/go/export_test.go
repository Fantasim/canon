package gogen

import "github.com/fantasim/canonlang/internal/ir"

// The causes of the dependent-type refusals, which tests match with errors.Is (go.md §3: never an error's text).
var (
	ErrDependentNoDisc   = errDependentNoDisc
	ErrDependentDisc     = errDependentDisc
	ErrDependentNested   = errDependentNested
	ErrDependentValue    = errDependentValue
	ErrDependentNoBranch = errDependentNoBranch
	ErrDependentNever    = errDependentNever
	ErrDependentUnion    = errDependentUnion
)

// GenerateWithPartLines is Generate with a smaller line budget of a function's body, so a test cuts a small package into parts (CODEGEN.md §2.11, DECISIONS 435).
func GenerateWithPartLines(p *ir.Package, e *ir.Emit, partLines int) ([]ir.File, error) {
	return generate(p, e, partLines)
}
