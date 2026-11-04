package types_test

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/types"
)

// A field's type is a tree of types; String prints it as Canon writes it, names qualified.
func Example() {
	statuses := &types.Collection{Kind: types.CollLet, Pkg: "teamboard", Name: "statuses"}
	previous, _ := types.Past(&types.RefType{Target: statuses}) // TYPES.md §8.4: may name a retired status
	status := &types.RecordType{Pkg: "teamboard", Name: "Status", Fields: []*types.Field{
		{Name: "label", Type: &types.Refined{Of: types.StringType, Range: &types.Bound{HasLo: true, Lo: types.Limit{I: 1}}}},
		{Name: "next", Type: &types.ListType{Elem: &types.RefType{Target: statuses}}},
		{Name: "previous", Type: &types.OptionalType{Elem: previous}},
		{Name: "limit", Type: &types.OptionalType{Elem: types.UInt16Type}},
		{Name: "timeout", Type: types.DurationType, Unit: types.UnitS},
	}}
	statuses.Elem = status
	for _, f := range status.Fields {
		fmt.Printf("%s: %s, ", f.Name, f.Type)
	}
	fmt.Println(types.FloatText(2.5e6, 64), types.DurationText(5_400_000), types.QuoteString("a\"b"))
	// Output: label: String(1..), next: [ref teamboard.statuses], previous: past ref teamboard.statuses?, limit: UInt16?, timeout: Duration, 2500000 1h30m "a\"b"
}

// A ref into a keyed list keyed by a ref is read at its chain's leaf key (WIRE.md §5.9).
func ExampleKeyChain() {
	boards := &types.Collection{Kind: types.CollLet, Pkg: "teamboard", Name: "boards"}
	lanes := &types.Collection{Kind: types.CollLet, Pkg: "teamboard", Name: "lanes", KeyedBy: &types.Field{Name: "id", Type: types.UInt16Type}}
	cards := &types.Collection{Kind: types.CollLet, Pkg: "teamboard", Name: "cards", KeyedBy: &types.Field{Name: "lane", Type: &types.RefType{Target: lanes}}}
	pins := &types.Collection{Kind: types.CollLet, Pkg: "teamboard", Name: "pins", KeyedBy: &types.Field{Name: "board", Type: &types.RefType{Target: boards}}}
	leaf := types.KeyChain(&types.RefType{Target: cards}, types.Resolved)
	table := types.KeyChain(&types.RefType{Target: pins}, types.Resolved)
	broken := types.CollChain(nil, types.Resolved)
	fmt.Println(leaf.Field.Name, leaf.Leaf, leaf.End == types.ChainLeaf, table.Leaf, table.End == types.ChainTable, broken.End == types.ChainBroken)
	// Output: id UInt16 true String true true
}
