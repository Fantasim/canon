package control

import (
	"context"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
)

// FoldWith is an Env.Fold that folds a constant of a `where` (C34) with f, on behalf of the
// declaration that writes it; nil without a folder.
func FoldWith(ctx context.Context, prog *check.Program, f check.Folder) func(syntax.Expr) (value.Value, bool) {
	if f == nil || prog == nil {
		return nil
	}
	return func(e syntax.Expr) (value.Value, bool) {
		owner := writer(prog, e)
		if owner == nil {
			return nil, false
		}
		return f.Fold(ctx, owner, e, prog.Info)
	}
}

// writer is the top-level declaration whose syntax holds e, nil for none.
func writer(prog *check.Program, e syntax.Expr) check.Object {
	for _, p := range prog.Packages {
		for _, o := range p.Decls {
			if o.Decl() != nil && holds(o.Decl(), e) {
				return o
			}
		}
	}
	return nil
}

// holds reports n or a node below it being e.
func holds(n syntax.Node, e syntax.Expr) bool {
	found := false
	syntax.Inspect(n, func(x syntax.Node) bool {
		found = found || x == syntax.Node(e)
		return !found
	})
	return found
}
