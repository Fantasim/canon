package gogen

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/ir"
)

// nameError is the error of the plan's first problem (CODEGEN.md §3.5): an override that is not exported names its item (decision 182), a name that is no Go identifier is ErrName, two names of one scope ErrNameCollision.
func nameError(pr ir.GoNameProblem) error {
	switch pr.Kind {
	case ir.GoUnexported:
		return newDetail(ErrName, pr.Origin, "@go(name: %q) of %s is not exported", pr.Name, pr.Origin)
	case ir.GoNotIdentifier:
		return fmt.Errorf("%w: %q (from %s)", ErrName, pr.Name, pr.Origin)
	case ir.GoCollision: // the default below
	}
	return fmt.Errorf("%w: %s %s: %s, %s", ErrNameCollision, pr.Scope, pr.Name, pr.First, pr.Origin)
}
