package edit

import (
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// frame is the frame around the value ty types (WIRE.md 5.9): its record's fields and
// parameters, the dependent map binders, with ty's applier for defaults and dereferences.
func (ty Typer) frame() depFrame {
	return depFrame{rec: ty.Outer.Record, params: ty.Outer.Params, binders: ty.Outer.Binders, a: ty.app}
}

// dependentValue is lit given to dependent type t: a list, record, map or case literal typed
// against the branch the record it stands in computes (API.md V1, DEP-02), else a name or a
// scalar a branch takes (symbol); one that computes Never takes no such literal (V1).
func (tc *typing) dependentValue(lit Lit, t types.Type) (value.Value, error) {
	switch lit.(type) {
	case List, Obj, Map, Case:
		bt, fr, ok := tc.fr.computedIn(t)
		switch {
		case !ok || computesNever(bt):
		case !tc.readable(t, bt, branchForm(lit)):
			return nil, tc.mismatch(lit, t, detailOtherBranch)
		default:
			return tc.within(fr, func() (value.Value, error) { return tc.value(lit, bt) })
		}
	}
	return tc.symbol(lit, t)
}

// branchForm is the test of the branches the checker reads lit, written as Canon text, against:
// a list's a list branch, a record's, map's or case's a brace branch (DECISIONS 171).
func branchForm(lit Lit) func(types.Type) bool {
	if _, isList := lit.(List); isList {
		return check.IsListBranch
	}
	return check.IsBraceBranch
}

// textForm is branchForm for a Source's expression e: a typed literal is printed as a brace's
// case or record (M7), so the brace test; nil for a scalar.
func textForm(e syntax.Expr) func(types.Type) bool {
	if _, typed := syntax.Unparen(e).(*syntax.TypedLit); typed {
		return check.IsBraceBranch
	}
	return formTest(e)
}

// readable reports bt, the branch dependent t computes, the one the checker reads a literal of
// form fits as: a Canon destination writes no other until typed literals do (EDIT-TYPEDSOURCE);
// a JSON one's decoder reads the computed branch (WIRE.md 5.9).
func (tc *typing) readable(t, bt types.Type, fits func(types.Type) bool) bool {
	if tc.ty.json || fits == nil {
		return true
	}
	return types.Identical(present(check.FirstFittingBranch(t, fits)), present(bt))
}

// computesNever reports a computed type that is Never, or Never?.
func computesNever(t types.Type) bool {
	return present(t).Base().Kind() == types.Never
}

// within runs typeIt in frame fr, the frame before restored after.
func (tc *typing) within(fr depFrame, typeIt func() (value.Value, error)) (value.Value, error) {
	was := tc.fr
	tc.fr = fr
	defer func() { tc.fr = was }()
	return typeIt()
}

// dependentExpr is e, no bare name, given to dependent type t (API.md V1; TYPES.md 11.4): typed
// against its form's branch, else a literal a branch takes kept as written.
func (st *srcTyping) dependentExpr(e syntax.Expr, t types.Type) (value.Value, error) {
	if bt, fr, ok := st.formBranch(e, t); ok {
		if !st.readable(t, bt, textForm(e)) {
			return nil, st.wrong(e, t, detailOtherBranch)
		}
		return st.within(fr, func() (value.Value, error) { return st.expr(e, bt) })
	}
	if v, ok := st.asWritten(e, t); ok {
		st.spell(e, v)
		return v, nil
	}
	return nil, st.wrong(e, t, detailDependent)
}

// formBranch is the branch a list, brace or typed literal e takes in dependent t: the one its
// record computes (V1), else a Canon list or brace literal's first (DECISIONS 171); never Never.
// A JSON destination has no first branch: its decoder always reads the computed one (WIRE.md 5.9).
func (st *srcTyping) formBranch(e syntax.Expr, t types.Type) (types.Type, depFrame, bool) {
	fits := formTest(e)
	if _, typed := syntax.Unparen(e).(*syntax.TypedLit); fits == nil && !typed {
		return nil, depFrame{}, false
	}
	bt, fr, computed := st.fr.computedIn(t)
	switch {
	case computed:
		return bt, fr, !computesNever(bt)
	case fits == nil || st.ty.json:
		return nil, depFrame{}, false
	}
	b := check.FirstFittingBranch(t, fits)
	return b, st.fr, !dependent(present(b))
}

// keyFrame is the frame of the value at key k of a map of type t: a dependent map's binder bound
// to k (TYPES.md 11.5), as depFrame.into binds it.
func (tc *typing) keyFrame(t types.Type, k value.Value) depFrame {
	if dm, ok := baseOf(t).(*types.DepMapType); ok {
		return tc.fr.bind(dm.Binder, k)
	}
	return tc.fr
}
