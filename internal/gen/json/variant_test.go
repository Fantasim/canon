package jsongen_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/testkit/golden"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// shapes is a list of a variant whose cases have methods, precomputed and finite (WIRE.md §5.11).
func shapes() *ir.Package {
	size := newEnum("geo", "Size", "small", "large")
	shape := &types.VariantType{Pkg: "geo", Name: "Shape", Tag: "kind"}
	intField := func(name string) *types.Field {
		return &types.Field{Name: name, Type: types.IntType, Wire: name, WirePath: []string{name}}
	}
	circle := &types.CaseType{Variant: shape, Name: "circle", Wire: "circle", Fields: []*types.Field{intField("r")}}
	square := &types.CaseType{Variant: shape, Name: "square", Wire: "square", Index: 1, Fields: []*types.Field{intField("side")}}
	point := &types.CaseType{Variant: shape, Name: "point", Wire: "pt", Index: 2}
	shape.Cases = []*types.CaseType{circle, square, point}
	c := &value.Record{T: circle, Fields: []value.Value{num(2)}}
	s := &value.Record{T: square, Fields: []value.Value{num(3)}}
	pt := &value.Record{T: point}
	area := &ir.ExportFn{Name: "area", Kind: ir.FnPrecomputed, Result: tInt.ir, Instances: []*ir.Instance{{Recv: c, Result: num(12)}}}
	fits := &ir.ExportFn{Name: "fits", Kind: ir.FnLookup, Result: tBool.ir, Params: []*ir.Param{{Name: "size", Type: size.typ().ir}},
		Instances: []*ir.Instance{{Recv: s, Table: &ir.LookupTable{Domains: [][]value.Value{size.all()}, Cells: []value.Value{boolean(false), boolean(true)}}}}}
	shapeIR := &ir.Variant{Pkg: "geo", Name: "Shape", Tag: "kind", Cases: []*ir.Case{
		{Name: "circle", Wire: "circle", Fields: []*ir.Field{{Name: "r", WirePath: []string{"r"}, Type: tInt.ir}}, Methods: []*ir.ExportFn{area}},
		{Name: "square", Wire: "square", Fields: []*ir.Field{{Name: "side", WirePath: []string{"side"}, Type: tInt.ir}}, Methods: []*ir.ExportFn{fits}},
		{Name: "point", Wire: "pt"},
	}}
	elem := ir.TypeRef{Kind: types.Variant, Named: shapeIR}
	return &ir.Package{Name: "geo", Dir: "geo", Values: []*ir.Value{{Name: "shapes", Type: ir.TypeRef{Kind: types.List, Elem: &elem},
		V: &value.List{T: &types.ListType{Elem: shape}, Elems: []value.Value{c, s, pt}}}}}
}

// WIRE.md §5.6, §5.11: a case's `$` keys follow its fields, against testdata/shapes.txtar.
func TestCaseMethods(t *testing.T) {
	p := shapes()
	files := generate(t, p, jsonEmit("geo/data"))
	golden.Run(t, "testdata/shapes.txtar", func(*testing.T, golden.Case) []byte { return files[0].Content }, golden.Expected(files[0].Path))
}
