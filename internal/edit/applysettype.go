package edit

import (
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
			return firstBranch(t, fits)
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
		return listBranch
	case *syntax.BraceLit:
		return braceBranch
	}
	return nil
}

func listBranch(t types.Type) bool {
	_, ok := present(t).Base().(*types.ListType)
	return ok
}

// braceBranch reports a type a brace literal can state, as the checker's braceBranch does.
func braceBranch(t types.Type) bool {
	switch present(t).Base().(type) {
	case *types.RecordType, *types.AppliedRecord, *types.CaseType, *types.TableType, *types.MapType, *types.DepMapType:
		return true
	}
	return false
}

// firstBranch is the first branch of dependent t that fits, t itself when none does.
func firstBranch(t types.Type, fits func(types.Type) bool) types.Type {
	u := present(t).Base()
	if l, ok := u.(*types.LitUnionType); ok {
		u = present(l.Of).Base()
	}
	fn := depFn(u)
	if fn == nil {
		return t
	}
	w := &flatBranches{seen: map[*types.TypeFunc]bool{}}
	w.walk(fn)
	for _, b := range w.out {
		if fits(b) {
			return b
		}
	}
	return t
}

// depFn is the type function a type application or dependent union computes, nil for another type.
func depFn(t types.Type) *types.TypeFunc {
	if t == nil {
		return nil
	}
	switch b := present(t).Base().(type) {
	case *types.TypeAppType:
		return b.Fn
	case *types.DepUnionType:
		return b.Fn
	}
	return nil
}

// flatBranches is the checker's branchWalk (check/dependent.go): a type function's results in
// order, one that is itself a type function, not optional, flattened in; each function once.
type flatBranches struct {
	seen map[*types.TypeFunc]bool
	out  []types.Type
}

func (w *flatBranches) walk(fn *types.TypeFunc) {
	if w.seen[fn] {
		return
	}
	w.seen[fn] = true
	results := []types.Type{fn.Body}
	for _, a := range fn.Arms {
		results = append(results, a.Result)
	}
	for _, r := range results {
		switch inner := depFn(r); {
		case r == nil:
		case inner != nil && r.Base().Kind() != types.Optional:
			w.walk(inner)
		default:
			w.out = append(w.out, r)
		}
	}
}
