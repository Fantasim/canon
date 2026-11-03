package gogen_test

import (
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

const nestedUnionPkg = "gennestunion"

// nestedUnionPackage is one table whose field is a literal union over a ref into a list keyed by a ref into a list keyed by an enum whose second member, `second`, is written "z2" on the wire (CODEGEN.md §4.1, §5.9, DECISIONS 387, 456).
func nestedUnionPackage() *ir.Package {
	tone := &ir.Enum{Pkg: nestedUnionPkg, Name: "Tone", Members: []*ir.EnumMember{
		{Name: "first", Wire: "first", Index: 0}, {Name: "second", Wire: "z2", Index: 1},
	}}
	inner := ir.TypeRef{Kind: types.Ref, Key: &ir.TypeRef{Kind: types.Enum, Named: tone}, Ref: &ir.RefTarget{Coll: types.CollLet, Pkg: nestedUnionPkg, Value: "aa", Keyed: true}}
	outer := ir.TypeRef{Kind: types.Ref, Key: &inner, Ref: &ir.RefTarget{Coll: types.CollLet, Pkg: nestedUnionPkg, Value: "bs", Keyed: true}}
	litT := ir.TypeRef{Kind: types.LitUnion, Elem: &outer, Literals: []string{"none"}}
	item := &ir.Record{Pkg: nestedUnionPkg, Name: "Item", Fields: []*ir.Field{{Name: "pick", WirePath: []string{"pick"}, Type: litT}}}
	itemT := &types.RecordType{Pkg: nestedUnionPkg, Name: "Item", Fields: []*types.Field{{Name: "pick", Index: 0, Wire: "pick", WirePath: []string{"pick"}}}}
	entry := func(id string, v value.Value) *value.Record {
		return &value.Record{T: itemT, Ident: &value.Identity{Key: value.Key{S: id}}, Fields: []value.Value{v}}
	}
	table := ir.TypeRef{Kind: types.Table, Elem: &ir.TypeRef{Kind: types.Record, Named: item}}
	return &ir.Package{
		Name: nestedUnionPkg, Dir: nestedUnionPkg, Types: []ir.Type{tone, item},
		Values: []*ir.Value{{Name: "items", Type: table, IDs: []string{"a", "b"}, V: &value.Table{Entries: []*value.Record{
			entry("a", &value.Ref{Key: value.Key{S: "second"}}), entry("b", &value.Str{V: "none"}),
		}}}},
		Emits: []*ir.Emit{{
			Target: ir.TargetGo, Out: "out/go/", Dir: nestedUnionPkg + "/out/go",
			GoImport: dataModule + "/" + nestedUnionPkg + "/out/go", Mode: ir.ModeBaked, GoPackage: nestedUnionPkg,
		}},
	}
}

// CODEGEN.md §4.1, §5.9, DECISIONS 387, 456: a union literal whose ref arm points into a list keyed by a ref is written as the wire of the key that ends the chain ("z2"), not the member's name, and the generated package compiles and reads it back.
func TestNestedRefUnionWire(t *testing.T) {
	files := generateData(t, nestedUnionPackage())
	for _, path := range slices.Sorted(maps.Keys(files)) {
		if strings.HasSuffix(path, ".gen.go") && !strings.Contains(string(files[path]), `"z2"`) {
			t.Errorf("%s does not hold the wire z2:\n%s", path, files[path])
		}
	}
	runData(t, files, nestedUnionPkg+"/out/go", "testdata/smoke/nestedref_union_test.go", nil)
}
