package cppgen_test

import (
	"errors"
	"testing"

	cppgen "github.com/fantasim/canonlang/internal/gen/cpp"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// thingType is the checker's type of small's record Thing, one field a: what its values name.
var thingType = &types.RecordType{Pkg: "demo", Name: "Thing", Fields: []*types.Field{{Name: "a"}}}

// withThings makes the emit baked and gives it a table things of Thing, one entry x.
func withThings(p *ir.Package, e *ir.Emit) {
	e.Mode = ir.ModeBaked
	elem := ir.TypeRef{Kind: types.Record, Named: thing(p)}
	coll := &types.Collection{Kind: types.CollLet, Pkg: "demo", Name: "things"}
	x := &value.Record{T: thingType, Fields: []value.Value{num(1)}, Ident: &value.Identity{Coll: coll, Key: value.Key{S: "x"}}}
	p.Values = append(p.Values, &ir.Value{Name: "things", IDs: []string{"x"}, Type: ir.TypeRef{Kind: types.Table, Elem: &elem}, V: &value.Table{Entries: []*value.Record{x}}})
}

// thingRef is a ref into things.
func thingRef(p *ir.Package) ir.TypeRef {
	return ir.TypeRef{Kind: types.Ref, Key: &tString, Ref: &ir.RefTarget{Coll: types.CollLet, Pkg: "demo", Value: "things", Elem: thing(p)}}
}

// bakedRefusals: what a baked emit finds malformed, each row unreachable past the check finding it names (CODEGEN.md §2.1, §5.9).
func bakedRefusals() []struct {
	name string
	edit func(*ir.Package, *ir.Emit)
	want error
} {
	return []struct {
		name string
		edit func(*ir.Package, *ir.Emit)
		want error
	}{
		// unreachable: stage E refuses it first (E8202).
		{"a @reload value", func(p *ir.Package, e *ir.Emit) { e.Mode = ir.ModeBaked; withValues(p, true, "things") }, cppgen.ErrMalformed},
		// unreachable: verification refuses a ref to no entry first (E3501).
		{"a ref to no entry", func(p *ir.Package, e *ir.Emit) {
			withThings(p, e)
			p.Values = append(p.Values, &ir.Value{Name: "r", Type: thingRef(p), V: &value.Ref{Key: value.Key{S: "nope"}}})
		}, cppgen.ErrMalformed},
		// unreachable: stage E precomputes every receiver's result (EVALUATION.md §2.3).
		{"a stored method without a result", func(p *ir.Package, e *ir.Emit) {
			withThings(p, e)
			thing(p).Methods = append(thing(p).Methods, &ir.ExportFn{Name: "big", Kind: ir.FnPrecomputed, Result: tBool})
		}, cppgen.ErrMalformed},
		// unreachable: stage E fills every cell of the domains (CODEGEN.md §5.10).
		{"a lookup whose cells miss a domain value", func(p *ir.Package, e *ir.Emit) {
			withThings(p, e)
			p.Fns = append(p.Fns, &ir.ExportFn{Name: "f", Kind: ir.FnLookup, Result: tInt, Params: []*ir.Param{{Name: "b", Type: tBool}}, Table: &ir.LookupTable{Cells: []value.Value{num(1)}}})
		}, cppgen.ErrMalformed},
	}
}

// TestBakedRefusals: each refusal of a baked emit names its cause through a sentinel.
func TestBakedRefusals(t *testing.T) {
	for _, c := range bakedRefusals() {
		t.Run(c.name, func(t *testing.T) {
			p, e := small(field("a", "a", "", tInt))
			c.edit(p, e)
			if _, err := cppgen.Generate(p, e); !errors.Is(err, c.want) {
				t.Errorf("Generate: %v, want %v", err, c.want)
			}
		})
	}
}
