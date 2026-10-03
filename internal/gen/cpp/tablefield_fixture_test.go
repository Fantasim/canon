package cppgen_test

import (
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

const (
	tablesPkg     = "demo.tables"
	tablesSchema  = "demo.tables.Shelf@0000000c"
	tableTypesPkg = "demo.tabletypes"
)

// tablesPackage is demo.tables in data mode: a table `shelves` of Shelf, each Shelf with a `table Slot` field and an optional one, each Slot with a ref to its shelf: the rows of a nested table resolve their refs at load (CODEGEN.md §4.2, §5.8; WIRE.md §5.7).
func tablesPackage() *ir.Package {
	slot := &ir.Record{Pkg: tablesPkg, Name: "Slot", Doc: "A slot."}
	shelf := &ir.Record{Pkg: tablesPkg, Name: "Shelf", Doc: "A shelf."}
	slotElem := ir.TypeRef{Kind: types.Record, Named: slot}
	table := ir.TypeRef{Kind: types.Table, Elem: &slotElem}
	key := tString
	home := ir.TypeRef{Kind: types.Ref, Key: &key, Ref: &ir.RefTarget{Coll: types.CollLet, Pkg: tablesPkg, Value: "shelves", Elem: shelf}}
	slot.Fields = []*ir.Field{field("n", "n", "How many.", tInt), field("home", "home", "The shelf it is on.", home)}
	spare := field("spare", "spare", "Spare slots, if any.", table)
	spare.Optional = true
	shelf.Fields = []*ir.Field{field("slots", "slots", "The slots by id.", table), spare}
	shelfElem := ir.TypeRef{Kind: types.Record, Named: shelf}
	shelves := &ir.Value{Name: "shelves", Schema: tablesSchema, Doc: "The shelves.", Type: ir.TypeRef{Kind: types.Table, Elem: &shelfElem}}
	return &ir.Package{
		Name: tablesPkg, Dir: "demo/tables", Types: []ir.Type{slot, shelf}, Values: []*ir.Value{shelves},
		Emits: []*ir.Emit{{Target: ir.TargetCpp, Out: "out/", Dir: "demo/tables/out", Mode: ir.ModeData, Namespace: "demo::tables"}},
	}
}

// tableTypesPackage is demo.tabletypes in types mode: a Shelf whose `table Slot` fields are read by its public Decode, one defaulting to an empty table when its key is absent (CODEGEN.md §5.13).
func tableTypesPackage() *ir.Package {
	slot := &ir.Record{Pkg: tableTypesPkg, Name: "Slot", Fields: []*ir.Field{field("n", "n", "", tInt)}}
	slotElem := ir.TypeRef{Kind: types.Record, Named: slot}
	table := ir.TypeRef{Kind: types.Table, Elem: &slotElem}
	extra := field("extra", "extra", "", table)
	extra.Default = &value.Table{}
	spare := field("spare", "spare", "", table)
	spare.Optional = true
	shelf := &ir.Record{Pkg: tableTypesPkg, Name: "Shelf", Fields: []*ir.Field{field("title", "title", "", tString), field("slots", "slots", "", table), extra, spare}}
	return &ir.Package{
		Name: tableTypesPkg, Dir: "demo/tabletypes", Types: []ir.Type{slot, shelf},
		Emits: []*ir.Emit{{Target: ir.TargetCpp, Out: "out/", Dir: "demo/tabletypes/out", Mode: ir.ModeTypes, Namespace: "demo::tabletypes"}},
	}
}
