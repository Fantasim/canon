package gogen_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

const mapFieldGoldens = "testdata/mapfields"

// mapFieldPkg is a record value Board holding map fields of every key kind a loader reads: an enum, an enum with @json(codes), a String, an integer and a sized one, a literal union, a ref into its own table and into a keyed list, nested maps, list and record values, and a stored fn's map (CODEGEN.md §5.9, WIRE.md §5.8, DECISIONS 312).
func mapFieldPkg(name string) *ir.Package {
	element := &ir.Enum{Pkg: name, Name: "Element", Doc: "An element.", Members: []*ir.EnumMember{{Name: "fire", Wire: "fire"}, {Name: "water", Wire: "water", Index: 1}}}
	mapOf := func(k, v ir.TypeRef) ir.TypeRef { return ir.TypeRef{Kind: types.Map, Key: &k, Elem: &v} }
	rank := &ir.Enum{Pkg: name, Name: "Rank", Doc: "A rank.", Codes: &u8T, JSONCodes: true, Members: []*ir.EnumMember{{Name: "low", Wire: "low", Code: 1}, {Name: "high", Wire: "high", Index: 1, Code: 200}}}
	tone := &ir.Enum{Pkg: name, Name: "Tone", Doc: "A tone.", Members: []*ir.EnumMember{{Name: "plain", Wire: "plain"}, {Name: "series_1", Wire: "series-1", Index: 1}}}
	board := &ir.Record{Pkg: name, Name: "Board", Doc: "A board."}
	nodeRef := ir.TypeRef{Kind: types.Ref, Key: &intT, Ref: &ir.RefTarget{Coll: types.CollLet, Pkg: name, Value: "nodes", Elem: board, Keyed: true}}
	home := ir.TypeRef{Kind: types.Ref, Key: &strT, Ref: &ir.RefTarget{Coll: types.CollLet, Pkg: name, Value: "boards", Elem: board}}
	slot := &ir.Record{Pkg: name, Name: "Slot", Doc: "A slot.", Fields: []*ir.Field{wired("n", "n", "How many.", intT), wired("home", "home", "The board it is on.", home)}}
	board.Fields = []*ir.Field{
		wired("power", "power", "The power by element.", mapOf(typed(element, types.Enum), intT)),
		wired("names", "names", "The names by key.", mapOf(strT, strT)),
		wired("levels", "levels", "The levels by number.", mapOf(intT, strT)),
		wired("groups", "groups", "The members by group.", mapOf(strT, listT(intT))),
		wired("slots", "slots", "The slots by name.", mapOf(strT, typed(slot, types.Record))),
		opt(wired("extra", "extra", "Extra counts, if any.", mapOf(strT, intT))),
		wired("ranks", "ranks", "The counts by rank, keyed by code.", mapOf(typed(rank, types.Enum), intT)),
		wired("tones", "tones", "The counts by tone or `all`.", mapOf(ir.TypeRef{Kind: types.LitUnion, Elem: &ir.TypeRef{Kind: types.Enum, Named: tone}, Literals: []string{"all"}}, intT)),
		wired("byLit", "byLit", "The slots by tone or `all`.", mapOf(ir.TypeRef{Kind: types.LitUnion, Elem: &ir.TypeRef{Kind: types.Enum, Named: tone}, Literals: []string{"all"}}, typed(slot, types.Record))),
		wired("moods", "moods", "The tones by name, or `all`.", mapOf(strT, ir.TypeRef{Kind: types.LitUnion, Elem: &ir.TypeRef{Kind: types.Enum, Named: tone}, Literals: []string{"all"}})),
		wired("small", "small", "The names by small number.", mapOf(i8T, strT)),
		wired("byBoard", "byBoard", "The counts by board.", mapOf(home, intT)),
		wired("byNode", "byNode", "The names by node.", mapOf(nodeRef, strT)),
		wired("deep", "deep", "Nested maps.", mapOf(strT, mapOf(intT, strT))),
		wired("rankSlots", "rankSlots", "The slots by rank.", mapOf(typed(rank, types.Enum), typed(slot, types.Record))),
		wired("elemSlots", "elemSlots", "The slots by element.", mapOf(typed(element, types.Enum), typed(slot, types.Record))),
	}
	board.Methods = []*ir.ExportFn{{Name: "totals", Kind: ir.FnPrecomputed, Result: mapOf(strT, intT), Doc: "The totals by name."}, {Name: "byBoardTotals", Kind: ir.FnPrecomputed, Result: mapOf(home, intT), Doc: "The totals by board."}, {Name: "perTone", Kind: ir.FnLookup, Result: mapOf(home, intT), Params: []*ir.Param{{Name: "t", Type: typed(tone, types.Enum)}}, Doc: "The counts by board, per tone."}}
	elem := typed(board, types.Record)
	v := &ir.Value{Name: "boards", Schema: name + ".Board@0000000a", IDs: []string{"b1", "b2"}, Type: ir.TypeRef{Kind: types.Table, Elem: &elem}}
	return &ir.Package{
		Name: name, Dir: name, Types: []ir.Type{element, rank, tone, slot, board}, Values: []*ir.Value{v},
		Emits: []*ir.Emit{{
			Target: ir.TargetGo, Out: "out/go/", Dir: name + "/out/go", GoImport: dataModule + "/" + name + "/out/go", Mode: ir.ModeData, GoPackage: name,
		}},
	}
}

// CODEGEN.md §5.9, WIRE.md §5.8, DECISIONS 312: a data loader reading map fields equals its golden.
func TestMapFieldGolden(t *testing.T) {
	files := generateData(t, mapFieldPkg("genmapdata"))
	checkGoldens(t, files, sortedPaths(files), mapFieldGoldens)
}

// WIRE.md §5.8, DECISIONS 312: a loader reads a map in file order, keys by their wire form, and refuses a key its type does not read.
func TestMapFieldDataRuns(t *testing.T) {
	files := generateData(t, mapFieldPkg("genmapdata"))
	runData(t, files, "genmapdata/out/go", "testdata/smoke/mapfield_data_test.go", readData(t, "testdata/datafiles/mapfields"))
}
