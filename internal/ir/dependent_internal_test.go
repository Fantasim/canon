package ir

import (
	"testing"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// bakedCase is a record Thing {k: K, ev: ref, p: <typ>} holding p's value v, of package pkg's dependent type P.
type bakedCase struct {
	name string
	typ  func(app TypeRef) TypeRef
	from []string // p's argument, a wire path
	pkg  string
	v    value.Value
	want bool
}

// TestBakedDependent is CODEGEN.md §5.6 and decision 37: baked gen/go writes a dependent value only as a field value, or its list's elements, whose discriminant its record's fields hold (DiscFields), of its own package's type; anything else is refused at stage E.
func TestBakedDependent(t *testing.T) {
	str := TypeRef{Kind: types.String}
	cases := []bakedCase{
		{"an earlier enum field", nil, []string{"k"}, "a", &value.Int{V: 1}, false},
		{"a list sharing the field's discriminant", listOf, []string{"k"}, "a", &value.List{Elems: []value.Value{&value.Int{V: 1}}}, false},
		{"none", nil, []string{"ev"}, "a", &value.None{}, false},
		{"an empty list", listOf, []string{"ev"}, "a", &value.List{}, false},
		{"a discriminant read through a ref (WIRE.md §5.9)", nil, []string{"ev"}, "a", &value.Int{V: 1}, true},
		{"another package's type", nil, []string{"k"}, "b", &value.Int{V: 1}, true},
		{"a map's value", func(app TypeRef) TypeRef { return TypeRef{Kind: types.Map, Key: &str, Elem: &app} },
			[]string{"k"}, "a", &value.Map{Keys: []value.Value{&value.Str{V: "x"}}, Vals: []value.Value{&value.Int{V: 1}}}, true},
		{"a union's string value", unionOf, []string{"k"}, "a", &value.Str{V: "x"}, false},
		{"a union's member value", unionOf, []string{"k"}, "a", &value.Member{Index: 0}, true},
	}
	for _, c := range cases {
		thing, row := bakedThing(c)
		if got := bakedDependent("a", &thing, row); got != c.want {
			t.Errorf("%s: bakedDependent = %v, want %v", c.name, got, c.want)
		}
	}
}

func listOf(app TypeRef) TypeRef { return TypeRef{Kind: types.List, Elem: &app} }

func unionOf(app TypeRef) TypeRef {
	return TypeRef{Kind: types.LitUnion, Elem: &app, Literals: []string{"none"}}
}

// bakedThing is c's record type and its value: k is one, ev refers to entry x.
func bakedThing(c bakedCase) (TypeRef, *value.Record) {
	k := &Enum{Pkg: "a", Name: "K", Members: []*EnumMember{{Name: "one", Wire: "one"}}}
	kT := TypeRef{Kind: types.Enum, Named: k}
	d := &Dependent{Pkg: c.pkg, Name: "P", Params: 1, Disc: &kT, ByMember: []int{0},
		Branches: []*Branch{{Name: "one", Members: []int{0}, Type: TypeRef{Kind: types.Int, Bits: 64, Signed: true}}}}
	if c.from[0] == "ev" {
		d.DiscPath = []string{"k"}
	}
	app := TypeRef{Kind: types.TypeApp, Named: d, Args: []*Source{{From: types.ArgField, WirePath: c.from}}}
	if c.typ != nil {
		app = c.typ(app)
	}
	str := TypeRef{Kind: types.String}
	rec := &Record{Pkg: "a", Name: "Thing", Fields: []*Field{
		{Name: "k", WirePath: []string{"k"}, Type: kT},
		{Name: "ev", WirePath: []string{"ev"}, Type: TypeRef{Kind: types.Ref, Key: &str, Ref: &RefTarget{Coll: types.CollLet, Pkg: "a", Value: "evs"}}},
		{Name: "p", WirePath: []string{"p"}, Type: app},
	}}
	rt := &types.RecordType{Pkg: "a", Name: "Thing", Fields: []*types.Field{{Name: "k"}, {Name: "ev", Index: 1}, {Name: "p", Index: 2}}}
	row := &value.Record{T: rt, Fields: []value.Value{&value.Member{Index: 0}, &value.Ref{Key: value.Key{S: "x"}}, c.v}}
	return TypeRef{Kind: types.Record, Named: rec}, row
}
