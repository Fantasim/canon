package edit

import (
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/views/shape"
)

// isLoadDir reports a load whose method is no format and not load.defines: load.dir.
func isLoadDir(info *check.Info, e syntax.Expr) bool {
	x, ok := e.(*syntax.LoadExpr)
	return ok && x.Method != nil && shape.SourceForm(info, x) == shape.FormJSON
}

// initializer is the expression of a top-level let or const.
func initializer(n syntax.Node) syntax.Expr {
	switch d := n.(type) {
	case *syntax.LetDecl:
		return d.Value
	case *syntax.ConstDecl:
		return d.Value
	}
	return nil
}
