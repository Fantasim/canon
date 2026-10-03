package gogen_test

import (
	"errors"
	"testing"

	gogen "github.com/fantasim/canonlang/internal/gen/go"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

const tableFieldGoldens = "testdata/tablefields"

// tableFieldPkg is a record value Shelf with a `table Slot` field and an optional one: the rows of a nested table, with and without a retired entry (CODEGEN.md §4.2).
func tableFieldPkg(name string, mode ir.Mode) *ir.Package {
	slot := &ir.Record{Pkg: name, Name: "Slot", Doc: "A slot.", Fields: []*ir.Field{wired("n", "n", "How many.", intT)}}
	slotRef := ir.TypeRef{Kind: types.Record, Named: slot}
	table := ir.TypeRef{Kind: types.Table, Elem: &slotRef}
	shelf := &ir.Record{Pkg: name, Name: "Shelf", Doc: "A shelf.", Fields: []*ir.Field{
		wired("title", "title", "", strT),
		wired("slots", "slots", "The slots by id.", table),
		opt(wired("spare", "spare", "Spare slots, if any.", table)),
		wired("fav", "fav", "The favourite slot.", slotRefT(name, slot)),
		opt(wired("next", "next", "The next slot, if any.", slotRefT(name, slot))),
	}}
	slotT := &types.RecordType{Pkg: name, Name: "Slot", Fields: []*types.Field{{Name: "n", Index: 0, Wire: "n", WirePath: []string{"n"}}}}
	shelfT := &types.RecordType{Pkg: name, Name: "Shelf", Fields: []*types.Field{
		{Name: "title", Index: 0, Wire: "title", WirePath: []string{"title"}},
		{Name: "slots", Index: 1, Wire: "slots", WirePath: []string{"slots"}},
		{Name: "spare", Index: 2, Wire: "spare", WirePath: []string{"spare"}},
		{Name: "fav", Index: 3, Wire: "fav", WirePath: []string{"fav"}},
		{Name: "next", Index: 4, Wire: "next", WirePath: []string{"next"}},
	}}
	row := func(id string, retired bool, n int64) *value.Record {
		return &value.Record{T: slotT, Ident: &value.Identity{Key: value.Key{S: id}, Retired: retired}, Fields: []value.Value{&value.Int{V: n}}}
	}
	shelfV := &value.Record{T: shelfT, Fields: []value.Value{
		&value.Str{V: "front"},
		&value.Table{Entries: []*value.Record{row("a", false, 1), row("b", true, 2)}},
		&value.Table{Entries: []*value.Record{row("c", false, 3)}},
		&value.Ref{Key: value.Key{S: "b"}},
		&value.Ref{Key: value.Key{S: "a"}},
	}}
	v := &ir.Value{Name: "shelf", Schema: name + ".Shelf@0000000a", Type: ir.TypeRef{Kind: types.Record, Named: shelf}, V: shelfV}
	return &ir.Package{
		Name: name, Dir: name, Types: []ir.Type{slot, shelf}, Values: []*ir.Value{v},
		Emits: []*ir.Emit{{
			Target: ir.TargetGo, Out: "out/go/", Dir: name + "/out/go", GoImport: dataModule + "/" + name + "/out/go", Mode: mode, GoPackage: name,
		}},
	}
}

// slotRefT is a ref into the `slots` table field of Shelf: its key is the field's id type (DECISIONS 288).
func slotRefT(pkg string, slot *ir.Record) ir.TypeRef {
	return ir.TypeRef{Kind: types.Ref, Key: &strT, Ref: &ir.RefTarget{Coll: types.CollField, Pkg: pkg, Path: []string{"slots"}, Elem: slot}}
}

// CODEGEN.md §4.2, §5.3, WIRE.md §5.7: a table field in baked and data mode equals its golden.
func TestTableFieldGolden(t *testing.T) {
	files := generateData(t, tableFieldPkg("gentablebaked", ir.ModeBaked), tableFieldPkg("gentabledata", ir.ModeData))
	checkGoldens(t, files, sortedPaths(files), tableFieldGoldens)
}

// CODEGEN.md §4.2: baked Go builds a nested table's rows with their ids and retired flags, and compiles.
func TestTableFieldBakedCompiles(t *testing.T) {
	files := generateData(t, tableFieldPkg("gentablebaked", ir.ModeBaked))
	runData(t, files, "gentablebaked/out/go", "testdata/smoke/tablefield_baked_test.go", nil)
}

// WIRE.md §5.7: a data loader reads a nested table's object in key order, `$retired` and its refusals.
func TestTableFieldDataRuns(t *testing.T) {
	files := generateData(t, tableFieldPkg("gentabledata", ir.ModeData))
	runData(t, files, "gentabledata/out/go", "testdata/smoke/tablefield_data_test.go", readData(t, "testdata/datafiles/tablefields"))
}

// CODEGEN.md §4.2, §5.3: what stage E refuses as E8019 `TableField` is ErrMalformed here: a table field of another package's record, and in baked mode of the record of a table value, whose id is an enum.
func TestTableFieldRefusals(t *testing.T) {
	foreign := &ir.Record{Pkg: "other", Name: "Far", Fields: []*ir.Field{wired("n", "n", "", intT)}}
	cases := []struct {
		name  string
		mode  ir.Mode
		elem  func(*ir.Package) *ir.Record
		value func(*ir.Package, *ir.Record)
	}{
		{"foreign record", ir.ModeData, func(*ir.Package) *ir.Record { return foreign }, nil},
		{"baked table value's record", ir.ModeBaked, func(p *ir.Package) *ir.Record { return p.Types[0].(*ir.Record) }, func(p *ir.Package, rec *ir.Record) {
			elem := ir.TypeRef{Kind: types.Record, Named: rec}
			p.Values = []*ir.Value{{Name: "slots", Type: ir.TypeRef{Kind: types.Table, Elem: &elem}, V: &value.Table{}}}
		}},
	}
	for _, c := range cases {
		p := tableFieldPkg("gentablerefused", c.mode)
		rec := c.elem(p)
		elem := ir.TypeRef{Kind: types.Record, Named: rec}
		p.Types[1].(*ir.Record).Fields[1].Type = ir.TypeRef{Kind: types.Table, Elem: &elem}
		if c.value != nil {
			c.value(p, rec)
		}
		if err := generateErr(p, nil); !errors.Is(err, gogen.ErrMalformed) {
			t.Errorf("%s: %v, want ErrMalformed", c.name, err)
		}
	}
}

// tableRefPkg is a table of shelves whose nested slots each ref their shelf: the loader resolves the refs of the rows of a nested table (CODEGEN.md §5.8).
func tableRefPkg() *ir.Package {
	const name = "gentableref"
	slot := &ir.Record{Pkg: name, Name: "Slot"}
	shelf := &ir.Record{Pkg: name, Name: "Shelf"}
	slotRef := ir.TypeRef{Kind: types.Record, Named: slot}
	slot.Fields = []*ir.Field{wired("n", "n", "", intT), wired("home", "home", "", refT(name, "shelves", shelf, false))}
	shelf.Fields = []*ir.Field{wired("slots", "slots", "", ir.TypeRef{Kind: types.Table, Elem: &slotRef})}
	shelfRef := ir.TypeRef{Kind: types.Record, Named: shelf}
	shelves := &ir.Value{Name: "shelves", Schema: name + ".Shelf@0000000b", Type: ir.TypeRef{Kind: types.Table, Elem: &shelfRef}}
	return &ir.Package{
		Name: name, Dir: name, Types: []ir.Type{slot, shelf}, Values: []*ir.Value{shelves},
		Emits: []*ir.Emit{{
			Target: ir.TargetGo, Out: "out/go/", Dir: name + "/out/go", GoImport: dataModule + "/" + name + "/out/go", Mode: ir.ModeData, GoPackage: name,
		}},
	}
}

// CODEGEN.md §5.8, WIRE.md §5.7: the refs inside the rows of a nested table resolve at load, a missing entry named by its row.
func TestTableFieldRefsRun(t *testing.T) {
	files := generateData(t, tableRefPkg())
	checkGoldens(t, files, sortedPaths(files), tableFieldGoldens)
	runData(t, files, "gentableref/out/go", "testdata/smoke/tablefield_ref_test.go", readData(t, "testdata/datafiles/tableref"))
}
