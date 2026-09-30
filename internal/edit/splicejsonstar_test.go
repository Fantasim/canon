package edit_test

import (
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// WIRE.md 6.3, 5.8, DECISIONS 175 (log-2026-09-29 M4 B11): a map written item by item under an
// `at:` `*` writes a held symbol key as the key it was read as, as mapIn does, not an error.
// No load reaches it today (a let's key type is never dependent): the writer is tested alone.
func TestStarNodeHeldKey(t *testing.T) {
	mt := &types.MapType{Key: types.StringType, Value: types.IntType}
	m := &value.Map{T: mt,
		Keys: []value.Value{&value.Str{V: "a", T: types.StringType}, &value.Symbol{Name: "zzz", T: types.StringType}},
		Vals: []value.Value{&value.Int{V: 1, T: types.IntType}, &value.Int{V: 2, T: types.IntType}},
	}
	keys, err := edit.StarKeys(m)
	if err != nil || !slices.Equal(keys, []string{"a", "zzz"}) {
		t.Errorf("keys %q, %v; want [a zzz]", keys, err)
	}
}
