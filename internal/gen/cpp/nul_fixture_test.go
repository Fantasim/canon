package cppgen_test

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

const (
	nulPkg  = "demo.nulstr"
	nulText = "a\x00b" // three bytes: a NUL a C++ literal's own length would stop at
)

// textSpec is a package of one name, directory and namespace whose strings all hold one text.
type textSpec struct{ pkg, dir, ns, text string }

// nulSpec is the baked-nul fixture's package.
var nulSpec = textSpec{nulPkg, "demo/nulstr", "demo::nulstr", nulText}

// strs are n strings, the first the spec's text, the others plain.
func (s textSpec) strs(n int) []value.Value {
	out := []value.Value{str(s.text)}
	for i := 1; i < n; i++ {
		out = append(out, str(fmt.Sprintf("s%d", i)))
	}
	return out
}

// strMap is a map of n entries keyed by strs.
func (s textSpec) strMap(n int) *value.Map {
	m := &value.Map{Keys: s.strs(n)}
	for i := range n {
		m.Vals = append(m.Vals, num(int64(i+1)))
	}
	return m
}

// bakedPackage holds the text in a field, short and long lists and map keys, constants and a constexpr cell (CODEGEN.md §3.4).
func (s textSpec) bakedPackage() *ir.Package {
	note := &ir.Record{Pkg: s.pkg, Name: "Note", Fields: []*ir.Field{field("text", "text", "", tString)}}
	noteT := bulkTypeOf(s.pkg, "Note", note)
	coll := &types.Collection{Kind: types.CollLet, Pkg: s.pkg, Name: "notes"}
	row := &value.Record{T: noteT, Fields: []value.Value{str(s.text)}, Ident: &value.Identity{Coll: coll, Key: value.Key{S: "n0"}}}
	noteElem := ir.TypeRef{Kind: types.Record, Named: note}
	k, v := tString, tInt
	tMap := ir.TypeRef{Kind: types.Map, Key: &k, Elem: &v}
	side := enumOf("Side", "left", "right")
	side.Pkg = s.pkg
	return &ir.Package{
		Name: s.pkg, Dir: s.dir, Types: []ir.Type{side, note},
		Values: []*ir.Value{
			{Name: "notes", IDs: []string{"n0"}, Type: ir.TypeRef{Kind: types.Table, Elem: &noteElem}, V: &value.Table{Entries: []*value.Record{row}}},
			{Name: "short", Type: listOf(tString), V: &value.List{Elems: s.strs(2)}},
			{Name: "long", Type: listOf(tString), V: &value.List{Elems: s.strs(smallN)}},
			{Name: "shortMap", Type: tMap, V: s.strMap(2)},
			{Name: "longMap", Type: tMap, V: s.strMap(smallN)},
		},
		Consts: []*ir.Const{
			{Name: "NUL_TEXT", Type: tString, V: str(s.text)},
			{Name: "NUL_SHORT", Type: listOf(tString), V: &value.List{Elems: s.strs(2)}},
			{Name: "NUL_LONG", Type: listOf(tString), V: &value.List{Elems: s.strs(smallN)}},
		},
		Fns:   []*ir.ExportFn{lookup("sideText", "", tString, []*ir.Param{{Name: "s", Type: ir.TypeRef{Kind: types.Enum, Named: side}}}, str(s.text), str("r"))},
		Emits: []*ir.Emit{{Target: ir.TargetCpp, Out: "out/", Dir: s.dir + "/out", Mode: ir.ModeBaked, Namespace: s.ns}},
	}
}

func nulPackage() *ir.Package { return nulSpec.bakedPackage() }

// nulTypesPackage is a types-mode package whose variant's case wire holds a NUL, which its decoder compares whole (CODEGEN.md §3.4, §5.13).
func nulTypesPackage() *ir.Package {
	prize := &ir.Variant{Pkg: nulPkg, Name: "Prize", Tag: "kind", Cases: []*ir.Case{
		{Name: "coins", Wire: nulText, Fields: []*ir.Field{field("amount", "amount", "", tInt)}}, {Name: "none", Wire: "none"},
	}}
	holder := &ir.Record{Pkg: nulPkg, Name: "Holder", Fields: []*ir.Field{field("prize", "prize", "", ir.TypeRef{Kind: types.Variant, Named: prize})}}
	return &ir.Package{
		Name: nulPkg, Dir: "demo/nulstr", Types: []ir.Type{prize, holder},
		Emits: []*ir.Emit{{Target: ir.TargetCpp, Out: "out/", Dir: "demo/nulstr/out", Mode: ir.ModeTypes, Namespace: "demo::nulstr"}},
	}
}
