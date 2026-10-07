package std_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/eval/std"
	"github.com/fantasim/canonlang/internal/types"
)

// ADR-0018: only methods returning a scalar or one element of their receiver's family peek.
func TestPeeks(t *testing.T) {
	list := &types.ListType{Elem: types.IntType}
	keyed := &types.ListType{Elem: types.IntType, KeyedBy: &types.Field{Name: "k"}}
	table := &types.TableType{Elem: types.IntType}
	m := &types.MapType{Key: types.StringType, Value: types.IntType}
	for _, c := range []struct {
		t    types.Type
		name string
		want bool
	}{
		{list, "len", true}, {list, "isEmpty", true}, {list, "get", true}, {list, "contains", true},
		{keyed, "get", true}, {keyed, "hasKey", true}, {table, "get", true}, {table, "hasKey", true},
		{m, "len", true}, {m, "isEmpty", true}, {m, "get", true}, {m, "contains", true},
		{list, "hasKey", false}, {m, "hasKey", false}, {types.StringType, "len", false}, {nil, "len", false},
		{m, "keys", false}, {m, "values", false}, {m, "filter", false}, {list, "map", false},
		{list, "reverse", false}, {list, "unique", false}, {table, "active", false}, {keyed, "sortBy", false},
	} {
		if got := std.Peeks(c.t, c.name); got != c.want {
			t.Errorf("Peeks(%v, %s) = %v, want %v", c.t, c.name, got, c.want)
		}
	}
}
