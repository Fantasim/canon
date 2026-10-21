package cppgen_test

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

const (
	nulPkg  = "demo.nul"
	nulText = "a\x00b" // three bytes: a NUL a C++ literal's own length would stop at
)

// nulStrs are n strings, the first nulText, the others plain.
func nulStrs(n int) []value.Value {
	out := []value.Value{str(nulText)}
	for i := 1; i < n; i++ {
		out = append(out, str(fmt.Sprintf("s%d", i)))
	}
	return out
}

// nulMap is a map of n entries keyed by nulStrs.
func nulMap(n int) *value.Map {
	m := &value.Map{Keys: nulStrs(n)}
	for i := range n {
		m.Vals = append(m.Vals, num(int64(i+1)))
	}
	return m
}

// nulPackage holds a NUL in a field, short and long lists and map keys, constants and a constexpr cell (CODEGEN.md §3.4).
func nulPackage() *ir.Package {
	note := &ir.Record{Pkg: nulPkg, Name: "Note", Fields: []*ir.Field{field("text", "text", "", tString)}}
	noteT := bulkTypeOf(nulPkg, "Note", note)
	coll := &types.Collection{Kind: types.CollLet, Pkg: nulPkg, Name: "notes"}
	row := &value.Record{T: noteT, Fields: []value.Value{str(nulText)}, Ident: &value.Identity{Coll: coll, Key: value.Key{S: "n0"}}}
	noteElem := ir.TypeRef{Kind: types.Record, Named: note}
	k, v := tString, tInt
	tMap := ir.TypeRef{Kind: types.Map, Key: &k, Elem: &v}
	side := enumOf("Side", "left", "right")
	side.Pkg = nulPkg
	return &ir.Package{
		Name: nulPkg, Dir: "demo/nul", Types: []ir.Type{side, note},
		Values: []*ir.Value{
			{Name: "notes", IDs: []string{"n0"}, Type: ir.TypeRef{Kind: types.Table, Elem: &noteElem}, V: &value.Table{Entries: []*value.Record{row}}},
			{Name: "short", Type: listOf(tString), V: &value.List{Elems: nulStrs(2)}},
			{Name: "long", Type: listOf(tString), V: &value.List{Elems: nulStrs(smallN)}},
			{Name: "shortMap", Type: tMap, V: nulMap(2)},
			{Name: "longMap", Type: tMap, V: nulMap(smallN)},
		},
		Consts: []*ir.Const{
			{Name: "NUL_TEXT", Type: tString, V: str(nulText)},
			{Name: "NUL_SHORT", Type: listOf(tString), V: &value.List{Elems: nulStrs(2)}},
			{Name: "NUL_LONG", Type: listOf(tString), V: &value.List{Elems: nulStrs(smallN)}},
		},
		Fns:   []*ir.ExportFn{lookup("sideText", "", tString, []*ir.Param{{Name: "s", Type: ir.TypeRef{Kind: types.Enum, Named: side}}}, str(nulText), str("r"))},
		Emits: []*ir.Emit{{Target: ir.TargetCpp, Out: "out/", Dir: "demo/nul/out", Mode: ir.ModeBaked, Namespace: "demo::nul"}},
	}
}

// nulTypesPackage is a types-mode package whose variant's case wire holds a NUL, which its decoder compares whole (CODEGEN.md §3.4, §5.13).
func nulTypesPackage() *ir.Package {
	prize := &ir.Variant{Pkg: nulPkg, Name: "Prize", Tag: "kind", Cases: []*ir.Case{
		{Name: "coins", Wire: nulText, Fields: []*ir.Field{field("amount", "amount", "", tInt)}}, {Name: "none", Wire: "none"},
	}}
	holder := &ir.Record{Pkg: nulPkg, Name: "Holder", Fields: []*ir.Field{field("prize", "prize", "", ir.TypeRef{Kind: types.Variant, Named: prize})}}
	return &ir.Package{
		Name: nulPkg, Dir: "demo/nul", Types: []ir.Type{prize, holder},
		Emits: []*ir.Emit{{Target: ir.TargetCpp, Out: "out/", Dir: "demo/nul/out", Mode: ir.ModeTypes, Namespace: "demo::nul"}},
	}
}
