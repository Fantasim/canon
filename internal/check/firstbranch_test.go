package check_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/types"
)

// DECISIONS 171, TYPES.md §11.4: the first branch of a dependent type that takes a literal's form (types built by hand; the literal-union cases are types E3002 refuses, TYPES.md §13.2, so they pin the helper, not reachable Canon).
func TestFirstFittingBranch(t *testing.T) {
	list := &types.ListType{Elem: types.IntType}
	optList := &types.OptionalType{Elem: list}
	mapOf := &types.MapType{Key: types.StringType, Value: types.IntType}
	fn := &types.TypeFunc{Arms: []*types.TypeArm{{Result: types.StringType}, {Result: optList}, {Result: mapOf}}}
	app := &types.TypeAppType{Fn: fn}
	for _, c := range []struct {
		name string
		t    types.Type
		fits func(types.Type) bool
		want types.Type
	}{
		{"application, list", app, check.IsListBranch, optList},
		{"application, brace", app, check.IsBraceBranch, mapOf},
		{"optional application", &types.OptionalType{Elem: app}, check.IsListBranch, optList},
		{"literal union over the application, list", &types.LitUnionType{Of: app, Literals: []string{"none"}}, check.IsListBranch, optList},
		{"literal union over the application, brace", &types.LitUnionType{Of: app, Literals: []string{"none"}}, check.IsBraceBranch, mapOf},
		{"dependent union", &types.DepUnionType{Fn: fn}, check.IsListBranch, optList},
		{"no branch fits", app, func(types.Type) bool { return false }, app},
		{"not dependent", list, check.IsListBranch, list},
	} {
		if got := check.FirstFittingBranch(c.t, c.fits); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

// DECISIONS 171: the form tests see through one optional layer, as the checker's literal does.
func TestBranchForms(t *testing.T) {
	list := &types.ListType{Elem: types.IntType}
	mapOf := &types.MapType{Key: types.StringType, Value: types.IntType}
	for _, c := range []struct {
		name        string
		t           types.Type
		list, brace bool
	}{
		{"list", list, true, false},
		{"optional list", &types.OptionalType{Elem: list}, true, false},
		{"map", mapOf, false, true},
		{"optional map", &types.OptionalType{Elem: mapOf}, false, true},
		{"string", types.StringType, false, false},
	} {
		if got := check.IsListBranch(c.t); got != c.list {
			t.Errorf("%s: IsListBranch = %v, want %v", c.name, got, c.list)
		}
		if got := check.IsBraceBranch(c.t); got != c.brace {
			t.Errorf("%s: IsBraceBranch = %v, want %v", c.name, got, c.brace)
		}
	}
}
