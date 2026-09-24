package load

import (
	"strings"

	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// plainString is a string literal with no interpolation: load reads its path and a plain default straight off the syntax tree, without the evaluator (SPEC.md §5.4).
func plainString(s *syntax.StringLit) (string, bool) {
	var b strings.Builder
	for _, p := range s.Parts {
		if p.Interp != nil {
			return "", false
		}
		b.WriteString(p.Text)
	}
	return b.String(), true
}

// defaultLiteralOK is whether e is a plain literal wireHost can read as is, of the same base
// kind as k: a mismatch (`x: Float = 1`) needs the evaluator, so it is refused this milestone
// (DECISIONS 173, meta/decisions/log-2026-09-24.md "load.dir review (M2)").
func defaultLiteralOK(e syntax.Expr, k types.Kind) bool {
	switch n := e.(type) {
	case *syntax.NoneLit:
		return true
	case *syntax.BoolLit:
		return k == types.Bool
	case *syntax.IntLit:
		return k == types.Int
	case *syntax.DurationLit:
		return k == types.Duration
	case *syntax.StringLit:
		_, ok := plainString(n)
		return ok && k == types.String
	default:
		return false
	}
}
