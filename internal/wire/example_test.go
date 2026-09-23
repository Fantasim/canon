package wire_test

import (
	"fmt"
	"strings"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/wire"
)

// An emit json data file: `$schema`, then one compact row per element with its `$` keys.
func Example() {
	heal := &types.Field{Name: "heal", Wire: "nHeal", WirePath: []string{"nHeal"}, Type: types.IntType}
	potion := &types.RecordType{Pkg: "pipeline", Name: "Potion", Fields: []*types.Field{heal}}
	potions := &value.List{T: &types.ListType{Elem: potion}, Elems: []value.Value{
		&value.Record{T: potion, Fields: []value.Value{&value.Int{V: 2500, T: types.IntType}}},
		&value.Record{T: potion, Fields: []value.Value{&value.Int{V: 500, T: types.IntType}}},
	}}
	isStrong := func(r *value.Record) []wire.Fn {
		return []wire.Fn{{Name: "isStrong", Result: &value.Bool{V: r.Fields[0].(*value.Int).V >= 2000}}}
	}
	doc := wire.Document{Schema: "pipeline.Potion@f750790e", Kind: types.List, V: potions, Methods: isStrong}
	b, err := doc.Encode()
	fmt.Println(strings.Split(string(b), "\n")[3], err)
	// Output: {"nHeal": 2500, "$isStrong": true}, <nil>
}
