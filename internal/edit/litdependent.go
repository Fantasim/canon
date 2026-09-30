package edit

import (
	"slices"

	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// asWritten is a string or integer literal e given to dependent type t, kept as written when a
// branch of t takes such a literal: verification converts it to the branch it computes (TYPES.md
// 11.4 "Literals", 11.6); false for any other expression, or a literal no branch takes (V1).
func (st *srcTyping) asWritten(e syntax.Expr, t types.Type) (value.Value, bool) {
	bt := branchTest{seen: map[*types.TypeFunc]bool{}}
	switch x := e.(type) {
	case *syntax.IntLit:
		if !x.Value.IsInt64() {
			return nil, false
		}
		n := x.Value.Int64()
		bt.fits = func(b types.Type) bool { return intTaken(n, b) }
		return &value.Int{V: n, T: types.IntType}, bt.takes(t)
	case *syntax.StringLit, *syntax.RawStringLit:
		s, ok := constString(e)
		if !ok {
			return nil, false
		}
		bt.fits = func(b types.Type) bool { return strTaken(s, b) }
		bt.union = func(lits []string) bool { return slices.Contains(lits, s) }
		return &value.Str{V: s, T: types.StringType}, bt.takes(t)
	}
	return nil, false
}

// branchTest asks whether a branch of a dependent type takes a literal: fits for a plain
// branch, union for a literal union's own literals (TYPES.md 13.2); seen stops at a type
// function met twice.
type branchTest struct {
	fits  func(types.Type) bool
	union func([]string) bool
	seen  map[*types.TypeFunc]bool
}

// takes reports t, or one of the branches it computes, taking the literal.
func (bt branchTest) takes(t types.Type) bool {
	switch b := present(t).Base().(type) {
	case *types.TypeAppType:
		return bt.fnTakes(b.Fn)
	case *types.DepUnionType:
		return bt.fnTakes(b.Fn)
	case *types.LitUnionType:
		return bt.union != nil && bt.union(b.Literals) || bt.takes(b.Of)
	}
	return bt.fits(present(t))
}

// fnTakes reports one of fn's results, its body or an arm's, taking the literal.
func (bt branchTest) fnTakes(fn *types.TypeFunc) bool {
	if bt.seen[fn] {
		return false
	}
	bt.seen[fn] = true
	results := []types.Type{fn.Body}
	for _, a := range fn.Arms {
		results = append(results, a.Result)
	}
	return slices.ContainsFunc(results, func(r types.Type) bool { return r != nil && bt.takes(r) })
}

// intTaken reports an integer literal n fitting branch b: an integer type, a Float, a ref into
// an integer-keyed collection (TYPES.md 4.1).
func intTaken(n int64, b types.Type) bool {
	_, isInt := intValue(n, b)
	_, isKey := refValue(b, func(kt types.Type) value.Value { return intKeyValue(n, kt) })
	return isInt || isKey
}

// strTaken reports a string literal s fitting branch b: a String, a ref into a collection keyed
// by strings (TYPES.md 4.1).
func strTaken(s string, b types.Type) bool {
	_, isStr := strValue(s, b)
	_, isKey := refValue(b, func(kt types.Type) value.Value { return textKeyValue(s, kt, false) })
	return isStr || isKey
}
