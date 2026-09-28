package eval

import (
	"fmt"
	"testing"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// DECISIONS 199: a lookup converts a symbol by the map's key type in O(1), reading none of its (nil) keys.
func TestKeyForReadsNoKey(t *testing.T) {
	color := &types.EnumType{Pkg: "a", Name: "Color", Members: []*types.Member{{Name: "red"}, {Name: "blue", Index: 1}}}
	dep := &types.DepUnionType{Fn: &types.TypeFunc{Pkg: "a", Name: "P"}}
	const keys = 100_000
	r := &run{ev: newEvaluator(nil, Options{})}
	cases := []struct {
		name string
		key  types.Type
		want string
	}{
		{"a key type verification computed", color, "*value.Member blue"},
		{"a stage-A map, its keys as written", dep, "*value.Symbol blue"},
	}
	for _, c := range cases {
		m := &value.Map{T: &types.MapType{Key: c.key, Value: types.IntType}, Keys: make([]value.Value, keys)}
		k := r.keyFor(m, &value.Symbol{Name: "blue"})
		if got := fmt.Sprintf("%T %s", k, k.CanonText()); got != c.want {
			t.Errorf("%s: %s, want %s", c.name, got, c.want)
		}
	}
}
