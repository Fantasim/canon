package shape_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/views/shape"
)

// TYPES.md §8.4, DECISIONS 304 (VIEWMODEL.md G8, G21): views compare static types, and statically `past X` is X; a written refinement still differs.
func TestSameTypePast(t *testing.T) {
	x := types.StringType
	past := &types.Refined{Of: x, Past: true}
	pat := &types.Refined{Of: x, Where: &types.Predicate{Text: "true"}}
	alias := &types.Alias{Name: "A", Def: past}
	for _, tc := range []struct {
		name string
		a, b types.Type
		want bool
	}{
		{"past X and X", past, x, true},
		{"X and past X", x, past, true},
		{"past of past", &types.Refined{Of: past, Past: true}, x, true},
		{"alias of past X and X", alias, x, true},
		{"past of a where and the where", &types.Refined{Of: pat, Past: true}, pat, true},
		{"past X and a where of X", past, pat, false},
		{"list of past X and list of X", &types.ListType{Elem: past}, &types.ListType{Elem: x}, true},
	} {
		if got := shape.SameType(tc.a, tc.b); got != tc.want {
			t.Errorf("%s: SameType = %v, want %v", tc.name, got, tc.want)
		}
	}
}
