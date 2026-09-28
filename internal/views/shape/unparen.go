package shape

import "github.com/fantasim/canonlang/internal/syntax"

// Unparen is e without its enclosing parentheses.
func Unparen(e syntax.Expr) syntax.Expr { return unparen(e) }
