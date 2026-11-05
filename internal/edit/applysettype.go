package edit

import (
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// setType is the type a Set's value must have (API.md V1): targetType, but where the arm cannot
// be computed a list, record, map or case literal keeps the dependent type, whose branch the
// typer then decides (formBranch: a Canon list or brace literal's first, DECISIONS 171, 411).
func (x *opCtx) setType() types.Type {
	t, dep := x.declaredType()
	if !dep {
		return t
	}
	if ct, computed := x.armOf(t, x.frameAt(len(x.res.Steps))); computed {
		return ct
	}
	switch lit := x.op.Value.(type) {
	case List, Obj, Map:
		return t
	case Case:
		if len(lit.Fields) > 0 {
			return t
		}
	case Source:
		if e, _ := parseLiteral(string(lit)); formTest(e) != nil {
			return t
		}
	}
	return x.heldType(t)
}

// formTest is the test of the branches list or brace literal e classifies against (DECISIONS
// 171), nil for another expression.
func formTest(e syntax.Expr) func(types.Type) bool {
	switch syntax.Unparen(e).(type) {
	case *syntax.ListLit:
		return check.IsListBranch
	case *syntax.BraceLit:
		return check.IsBraceBranch
	}
	return nil
}
