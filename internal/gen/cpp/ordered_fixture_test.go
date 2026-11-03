package cppgen_test

import (
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

const orderedPkg = "demo.ordered"

// orderedPackage is demo.ordered in types mode: a Shelf holding a map, a map of maps, a nested table and a list of racks that each hold a map, so the order of an ordered_json is read at every depth by the public Decode (CODEGEN.md §5.13; DECISIONS 437).
func orderedPackage() *ir.Package {
	slot := &ir.Record{Pkg: orderedPkg, Name: "Slot", Fields: []*ir.Field{field("n", "n", "", tInt)}}
	mapOf := func(k, v ir.TypeRef) ir.TypeRef { return ir.TypeRef{Kind: types.Map, Key: &k, Elem: &v} }
	slotElem := ir.TypeRef{Kind: types.Record, Named: slot}
	rack := &ir.Record{Pkg: orderedPkg, Name: "Rack", Fields: []*ir.Field{field("tags", "tags", "", mapOf(tString, tInt))}}
	rackElem := ir.TypeRef{Kind: types.Record, Named: rack}
	shelf := &ir.Record{Pkg: orderedPkg, Name: "Shelf", Fields: []*ir.Field{
		field("power", "power", "", mapOf(tString, tInt)),
		field("deep", "deep", "", mapOf(tString, mapOf(tString, tInt))),
		field("slots", "slots", "", ir.TypeRef{Kind: types.Table, Elem: &slotElem}),
		field("racks", "racks", "", ir.TypeRef{Kind: types.List, Elem: &rackElem}),
	}}
	return &ir.Package{
		Name: orderedPkg, Dir: "demo/ordered", Types: []ir.Type{slot, rack, shelf},
		Emits: []*ir.Emit{{Target: ir.TargetCpp, Out: "out/", Dir: "demo/ordered/out", Mode: ir.ModeTypes, Namespace: "demo::ordered"}},
	}
}
