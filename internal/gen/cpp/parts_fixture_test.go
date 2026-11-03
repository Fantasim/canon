package cppgen_test

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

const (
	partsPkg    = "demo.parts"
	partsInts   = 16 // Wide's Int members i0..
	partsText   = 8  // its String members s0..
	partsBools  = 4  // b0..
	partsFloats = 4  // f0..
	partsOpts   = 4  // its optional Int members o0..
	partsLists  = 2  // its list members l0..
	partsBlob   = 64 // the members of the variant case Blob, which alone passes partsLimit
	// partsLimit is the body budget the compile test cuts at: Wide's Decode is about 110 lines, Blob's 66.
	partsLimit = 60
)

// partsPackage is demo.parts in types mode: Wide has 40 members, a record default built into a static const and a variant whose case Blob is itself wider than partsLimit (CODEGEN.md §2.11, §7.6).
func partsPackage() *ir.Package {
	point := &ir.Record{Pkg: partsPkg, Name: "Point", Fields: []*ir.Field{field("x", "x", "", tInt), field("y", "y", "", tInt)}}
	blob := &ir.Case{Name: "blob", Wire: "blob"}
	for i := range partsBlob {
		name := fmt.Sprintf("p%d", i)
		blob.Fields = append(blob.Fields, field(name, name, "", tInt))
	}
	shape := &ir.Variant{Pkg: partsPkg, Name: "Shape", Tag: "kind", Cases: []*ir.Case{
		{Name: "dot", Wire: "dot", Fields: []*ir.Field{field("r", "r", "", tInt)}}, blob,
	}}
	var fields []*ir.Field
	numbered := func(prefix string, n int, t ir.TypeRef, opt func(i int, f *ir.Field)) {
		for i := range n {
			name := fmt.Sprintf("%s%d", prefix, i)
			f := field(name, name, "", t)
			opt(i, f)
			fields = append(fields, f)
		}
	}
	plain := func(int, *ir.Field) {}
	numbered("i", partsInts, tInt, func(i int, f *ir.Field) {
		if i == partsInts/2 {
			f.Default = num(9)
		}
	})
	numbered("s", partsText, tString, func(i int, f *ir.Field) {
		if i == 3 {
			f.Default = str("dflt")
		}
	})
	numbered("b", partsBools, tBool, plain)
	numbered("f", partsFloats, tFloat, plain)
	numbered("o", partsOpts, tInt, func(_ int, f *ir.Field) { f.Optional = true })
	numbered("l", partsLists, listOf(tInt), plain)
	at := field("at", "at", "", ir.TypeRef{Kind: types.Record, Named: point})
	at.Default = &value.Record{T: recordType("Point", "x", "y"), Fields: []value.Value{num(1), num(2)}}
	fields = append(fields, at, field("shape", "shape", "", ir.TypeRef{Kind: types.Variant, Named: shape}))
	wide := &ir.Record{Pkg: partsPkg, Name: "Wide", Fields: fields}
	return &ir.Package{
		Name: partsPkg, Dir: "demo/parts", Types: []ir.Type{point, shape, wide},
		Emits: []*ir.Emit{{Target: ir.TargetCpp, Out: "out/", Dir: "demo/parts/out", Mode: ir.ModeTypes, Namespace: "demo::parts"}},
	}
}
