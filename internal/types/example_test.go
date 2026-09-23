package types_test

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/types"
)

// A field's type is a tree of types; String prints it as Canon writes it, names qualified.
func Example() {
	statuses := &types.Collection{Kind: types.CollLet, Pkg: "teamboard", Name: "statuses"}
	status := &types.RecordType{Pkg: "teamboard", Name: "Status", Fields: []*types.Field{
		{Name: "label", Type: &types.Refined{Of: types.StringType, Range: &types.Bound{HasLo: true, Lo: types.Limit{I: 1}}}},
		{Name: "next", Type: &types.ListType{Elem: &types.RefType{Target: statuses}}},
		{Name: "limit", Type: &types.OptionalType{Elem: types.UInt16Type}},
		{Name: "timeout", Type: types.DurationType, Unit: types.UnitS},
	}}
	statuses.Elem = status
	for _, f := range status.Fields {
		fmt.Printf("%s: %s, ", f.Name, f.Type)
	}
	fmt.Println(types.FloatText(2.5e6, 64), types.DurationText(5_400_000), types.QuoteString("a\"b"))
	// Output: label: String(1..), next: [ref teamboard.statuses], limit: UInt16?, timeout: Duration, 2500000 1h30m "a\"b"
}
