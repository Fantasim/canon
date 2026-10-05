package format

import (
	"github.com/fantasim/canonlang/internal/syntax"
)

// FORMATTER.md §10, DECISIONS 319: a field `x: T? = none` written with an optional type.
func redundantNone(n *syntax.FieldDecl) bool {
	if _, none := n.Default.(*syntax.NoneLit); !none {
		return false
	}
	_, optional := n.Type.(*syntax.OptionalType)
	return optional
}

// FORMATTER.md §10, DECISIONS 319: the `=` and `none` of a redundant default are dropped tokens.
func dropRedundantNones(f *syntax.File, drop []bool) {
	syntax.Inspect(f, func(n syntax.Node) bool {
		fd, ok := n.(*syntax.FieldDecl)
		if !ok || !redundantNone(fd) {
			return true
		}
		last := fd.Default.First()
		drop[last] = true
		eq := last - 1
		for f.Tokens[eq].Kind == syntax.TokNL {
			eq--
		}
		drop[eq] = true
		return true
	})
}
