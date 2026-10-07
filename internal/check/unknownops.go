package check

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
)

// bothDependent is E2102 at the bare names of two context-dependent operands, else E3008 (TYPES.md §5.1, DECISIONS 333).
func (c *checker) bothDependent(env *env, e *syntax.BinaryExpr) {
	found := false
	for _, x := range [...]syntax.Expr{e.X, e.Y} {
		if namesSome(x) {
			c.synth(env, x)
			found = true
		}
	}
	if !found {
		c.report(env, diag.E3008.At(env.span(e)))
	}
}

// namesSome reports a bare name in a context-dependent operand (TYPES.md §5.1).
func namesSome(e syntax.Expr) bool {
	switch x := e.(type) {
	case *syntax.IdentExpr:
		return true
	case *syntax.ParenExpr:
		return namesSome(x.X)
	case *syntax.UnaryExpr:
		return namesSome(x.X)
	case *syntax.BinaryExpr:
		return namesSome(x.X) || namesSome(x.Y)
	default:
		return false
	}
}
