package ir

import (
	"testing"

	"github.com/fantasim/canonlang/internal/types"
)

// TYPES.md §8.4, DECISIONS 304: a `past` Refined checks nothing on a store, so a `let` of it translates; a written refinement still checks.
func TestStoreChecksPast(t *testing.T) {
	rng := &types.Bound{HasLo: true}
	for _, tc := range []struct {
		name string
		ty   types.Type
		want bool
	}{
		{"past alone", &types.Refined{Of: types.StringType, Past: true}, false},
		{"past of past", &types.Refined{Of: &types.Refined{Of: types.StringType, Past: true}, Past: true}, false},
		{"past of a range", &types.Refined{Of: &types.Refined{Of: types.IntType, Range: rng}, Past: true}, true},
		{"a range", &types.Refined{Of: types.IntType, Range: rng}, true},
		{"past of a sized Int", &types.Refined{Of: types.Basic{K: types.Int, Bits: 8, Signed: true}, Past: true}, true},
	} {
		if got := storeChecks(tc.ty); got != tc.want {
			t.Errorf("%s: storeChecks = %v, want %v", tc.name, got, tc.want)
		}
	}
}
