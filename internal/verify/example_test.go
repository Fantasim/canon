package verify_test

import (
	"context"
	"fmt"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/verify"
)

// Stage B walks `let deck: Deck = { maxHidden: 0 }` against `maxHidden: Int(1..)`, reports the
// value outside its range and marks it invalid, so its record's checks will be skipped.
func Example() {
	hidden := &types.Refined{Of: types.IntType, Range: &types.Bound{Lo: types.Limit{I: 1}, HasLo: true}}
	deck := &types.RecordType{Pkg: "teamboard", Name: "Deck", Fields: []*types.Field{{Name: "maxHidden", Type: hidden}}}
	zero := &value.Int{V: 0, T: hidden}
	ev := &evaluator{}
	bag := diag.NewBag(&source.FileSet{}, "teamboard")
	v := verify.New(ev, nil, map[string]*diag.Bag{"teamboard": bag}, nil)
	res, err := v.Check(context.Background(), eval.Root{Pkg: "teamboard", Name: "deck"}, &value.Record{T: deck, Fields: []value.Value{zero}})
	f := bag.Findings()[0]
	fmt.Println(res.Valid, err, f.Code, f.Path, ev.invalid[0] == zero)
	// Output: false <nil> E3204 deck.maxHidden true
}
