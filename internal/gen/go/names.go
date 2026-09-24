package gogen

import "github.com/fantasim/canonlang/internal/ir"

// nameError is ErrMalformed: a plan Problem reaching the generator is an internal defect (CODEGEN.md §3.5, go.md §3).
func nameError(pr ir.GoNameProblem) error {
	switch pr.Kind {
	case ir.GoUnexported:
		return newDetail(ErrMalformed, pr.Origin, "@go(name: %q) of %s is not exported", pr.Name, pr.Origin)
	case ir.GoNotIdentifier:
		return newDetail(ErrMalformed, pr.Origin, "%q (from %s) is no Go identifier", pr.Name, pr.Origin)
	default: // ir.GoCollision
		return newDetail(ErrMalformed, pr.Name, "%s %s: %s, %s", pr.Scope, pr.Name, pr.First, pr.Origin)
	}
}
