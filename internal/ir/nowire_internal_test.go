package ir

import (
	"testing"

	"github.com/fantasim/canonlang/internal/types"
)

// WIRE.md §5.9, DECISIONS 308: a Range, a function, a Pair and a variant kind have no wire form; a scalar and a list of one have.
func TestNoWire(t *testing.T) {
	reward := &types.VariantType{Name: "Reward"}
	for _, tc := range []struct {
		name string
		ty   types.Type
		want bool
	}{
		{"Range", types.RangeType, true},
		{"Pair", &types.PairType{A: types.IntType, B: types.IntType}, true},
		{"Kind(V)", &types.VariantKindType{Variant: reward}, true},
		{"Int", types.IntType, false},
		{"[Int]", &types.ListType{Elem: types.IntType}, false},
	} {
		if got := wireFind(tc.ty, map[types.Type]bool{}, noWire) != nil; got != tc.want {
			t.Errorf("%s: noWire = %v, want %v", tc.name, got, tc.want)
		}
	}
}
