package wire_test

import (
	"context"
	"fmt"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/jsonsrc"
	"github.com/fantasim/canonlang/internal/source"
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

// Decode reads a JSON source as its expected type, wire names and units applied; every
// mismatch is a finding located by its span and RFC 6901 pointer.
func ExampleDecoder_Decode() {
	heal := &types.Field{Name: "heal", Wire: "nHeal", WirePath: []string{"nHeal"}, Type: types.IntType}
	cooldown := &types.Field{Name: "cooldown", Index: 1, Wire: "dwCooldownSec", WirePath: []string{"dwCooldownSec"}, Type: types.DurationType, Unit: types.UnitS}
	potion := &types.RecordType{Pkg: "pipeline", Name: "Potion", Fields: []*types.Field{heal, cooldown}}
	var fs source.FileSet
	for _, text := range []string{`{"nHeal": 2500, "dwCooldownSec": 1.5}`, `{"nHeal": 2.5, "dwCooldownSec": 1}`} {
		f, _ := fs.Add("data/II_POT.json", "/p/data/II_POT.json", []byte(text))
		bag := diag.NewBag(&fs, "pipeline")
		root, _ := jsonsrc.Parse(f, bag)
		dec := wire.Decoder{Bag: bag, Pkg: "pipeline"}
		v, ok, err := dec.Decode(context.Background(), wire.Selection{Node: root}, potion)
		if ok {
			fmt.Println(v.CanonText(), err)
		}
		for _, l := range diag.Locate(&fs, bag.Findings()) {
			fmt.Printf("%s %s:%d:%d %s: %s\n", l.Code, l.Loc.Path, l.Loc.Line, l.Loc.Col, l.Pointer, l.Message)
		}
	}
	// Output:
	// Potion{heal: 2500, cooldown: 1s500ms} <nil>
	// E7103 data/II_POT.json:1:11 /nHeal: 2.5 is not an integer
}
