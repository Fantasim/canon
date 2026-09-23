package value_test

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// A table entry carries an identity; a ref to it equals it, and both print their key.
func Example() {
	potion := &types.RecordType{Pkg: "pipeline", Name: "Potion", Fields: []*types.Field{{Name: "heal"}, {Name: "cooldown", Index: 1}}}
	potions := &types.Collection{Kind: types.CollLet, Pkg: "pipeline", Name: "potions", Elem: potion}
	at := &value.Prov{Kind: value.ProvJSON, Span: source.Span{File: 2, Start: 0, End: 41}, Pointer: "/heal"}
	heal := &value.Record{
		T:      potion,
		Fields: []value.Value{&value.Int{V: 500, T: types.IntType, P: at}, &value.Dur{Ms: 8000}},
		Ident:  &value.Identity{Coll: potions, Key: value.Key{S: "II_POT_HEAL_L"}},
	}
	ref := &value.Ref{T: &types.RefType{Target: potions}, Key: value.Key{S: "II_POT_HEAL_L"}}
	fmt.Println(heal.CanonText())
	fmt.Println(ref.CanonText(), value.Equal(ref, heal), heal.Fields[0].Prov().Pointer)
	// Output:
	// Potion{heal: 500, cooldown: 8s}
	// II_POT_HEAL_L true /heal
}
