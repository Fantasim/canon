package value_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// API.md P9: a key is written by the key type declared at its map, a literal-union literal quoted.
func TestPathKey(t *testing.T) {
	slot := &types.LitUnionType{Of: types.StringType, Literals: []string{"none"}}
	body := &types.TypeAppType{Fn: &types.TypeFunc{Name: "Slot", Body: slot}}
	match := &types.TypeAppType{Fn: &types.TypeFunc{Name: "P", Scrutinee: &types.Scrutinee{}, Arms: []*types.TypeArm{{Result: slot}}}}
	none := str("none")
	for _, c := range []struct {
		name string
		k    value.Value
		kt   types.Type
		want string
	}{
		{"literal", none, slot, `"none"`},
		{"other word", str("other"), slot, "other"},
		{"plain String", none, types.StringType, "none"},
		{"no declared type", none, nil, "none"},
		{"alias", none, &types.Alias{Name: "Slot", Def: slot}, `"none"`},
		{"refined", none, &types.Refined{Of: slot}, `"none"`},
		{"optional", none, &types.OptionalType{Elem: slot}, `"none"`},
		{"nested union", str("all"), &types.LitUnionType{Of: slot, Literals: []string{"all"}}, `"all"`},
		{"type function body", none, body, `"none"`},
		{"match application, computed in verification", none, match, "none"},
		{"not a word", str("two words"), types.StringType, `"two words"`},
		{"underscore", str("_"), types.StringType, `"_"`},
		{"integer", num(-7), types.IntType, "-7"},
		{"member", &value.Member{Enum: tone, Index: 1}, &types.LitUnionType{Of: tone, Literals: []string{"series_1"}}, "series_1"},
		{"ref", &value.Ref{T: refStatus, Key: value.Key{S: "open"}}, refStatus, "open"},
	} {
		if got := value.PathKey(c.k, c.kt); got != c.want {
			t.Errorf("%s: PathKey = %s, want %s", c.name, got, c.want)
		}
	}
}
