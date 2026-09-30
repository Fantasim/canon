package value_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// TYPES.md §11.2: a type-level match selects on an enum member, a case kind or a Bool, nothing else.
func TestArmIndex(t *testing.T) {
	kind := &types.VariantKindType{Variant: reward}
	for _, c := range []struct {
		name string
		v    value.Value
		want int
		ok   bool
	}{
		{"member", &value.Member{Enum: tone, Index: 1}, 1, true},
		{"case kind", &value.CaseKind{T: kind, Index: 1}, 1, true},
		{"false", &value.Bool{V: false}, 0, true},
		{"true", &value.Bool{V: true}, 1, true},
		{"string", str("series_1"), 0, false},
		{"integer", num(1), 0, false},
		{"nil", nil, 0, false},
	} {
		if got, ok := value.ArmIndex(c.v); got != c.want || ok != c.ok {
			t.Errorf("%s: ArmIndex = %d, %v; want %d, %v", c.name, got, ok, c.want, c.ok)
		}
	}
}
