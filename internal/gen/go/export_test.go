package gogen

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
