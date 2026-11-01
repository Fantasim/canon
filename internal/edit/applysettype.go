package edit

import (
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// setType is the type a Set's value must have (API.md V1): targetType, but where the arm cannot
// be computed a list, record, map or case literal has none to fit, and a Source's list or brace
// literal takes the first branch of its form, as the checker reads it written (DECISIONS 171).
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
		if fits := sourceForm(lit); fits != nil {
			return check.FirstFittingBranch(t, fits)
		}
	}
	return x.heldType(t)
}

// sourceForm is the test of the branches a Source's list or brace literal classifies against,
// nil for another expression.
func sourceForm(text Source) func(types.Type) bool {
	e, _ := parseLiteral(string(text))
	switch syntax.Unparen(e).(type) {
	case *syntax.ListLit:
		return check.IsListBranch
	case *syntax.BraceLit:
		return check.IsBraceBranch
	}
	return nil
}
