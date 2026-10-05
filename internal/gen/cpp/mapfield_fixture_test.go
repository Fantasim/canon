package cppgen_test

import (
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

var tU8 = ir.TypeRef{Kind: types.Int, Bits: 8}

const (
	mapsPkg    = "demo.maps"
	mapsSchema = "demo.maps.Board@0000000e"
)

// mapsPackage is demo.maps in data mode: a table `boards` of Board, each Board with map fields of every key kind a loader reads (an enum, an enum with @json(codes), a String, an integer and a sized one, a literal union, a ref into its own table and into a keyed list), list, record and map values, an optional map and a stored fn's; each Slot held by a map resolves its ref to a board at load, and a ref key is checked (CODEGEN.md §5.9, §5.8, WIRE.md §5.8, DECISIONS 312).
func mapsPackage() *ir.Package {
	element := &ir.Enum{Pkg: mapsPkg, Name: "Element", Doc: "An element.", Members: []*ir.EnumMember{{Name: "fire", Wire: "fire"}, {Name: "water", Wire: "water", Index: 1}}}
	rank := &ir.Enum{Pkg: mapsPkg, Name: "Rank", Doc: "A rank.", Codes: &tU8, JSONCodes: true, Members: []*ir.EnumMember{{Name: "low", Wire: "low", Code: 1}, {Name: "high", Wire: "high", Index: 1, Code: 200}}}
	tone := &ir.Enum{Pkg: mapsPkg, Name: "Tone", Doc: "A tone.", Members: []*ir.EnumMember{{Name: "plain", Wire: "plain"}, {Name: "series_1", Wire: "series-1", Index: 1}}}
	slot := &ir.Record{Pkg: mapsPkg, Name: "Slot", Doc: "A slot."}
	board := &ir.Record{Pkg: mapsPkg, Name: "Board", Doc: "A board."}
	mapOf := func(k, v ir.TypeRef) ir.TypeRef { return ir.TypeRef{Kind: types.Map, Key: &k, Elem: &v} }
	key := tString
	home := ir.TypeRef{Kind: types.Ref, Key: &key, Ref: &ir.RefTarget{Coll: types.CollLet, Pkg: mapsPkg, Value: "boards", Elem: board}}
	nodeRef := ir.TypeRef{Kind: types.Ref, Key: &tInt, Ref: &ir.RefTarget{Coll: types.CollLet, Pkg: mapsPkg, Value: "nodes", Elem: board, Keyed: true}}
	toneKey := ir.TypeRef{Kind: types.LitUnion, Elem: &ir.TypeRef{Kind: types.Enum, Named: tone}, Literals: []string{"all"}}
	list := ir.TypeRef{Kind: types.List, Elem: &tInt}
	slotT := ir.TypeRef{Kind: types.Record, Named: slot}
	slot.Fields = []*ir.Field{field("n", "n", "How many.", tInt), field("home", "home", "The board it is on.", home)}
	extra := field("extra", "extra", "Extra counts, if any.", mapOf(tString, tInt))
	extra.Optional = true
	board.Fields = []*ir.Field{
		field("power", "power", "The power by element.", mapOf(ir.TypeRef{Kind: types.Enum, Named: element}, tInt)),
		field("names", "names", "The names by key.", mapOf(tString, tString)),
		field("levels", "levels", "The levels by number.", mapOf(tInt, tString)),
		field("groups", "groups", "The members by group.", mapOf(tString, list)),
		field("slots", "slots", "The slots by name.", mapOf(tString, slotT)),
		extra,
		field("ranks", "ranks", "The counts by rank, keyed by code.", mapOf(ir.TypeRef{Kind: types.Enum, Named: rank}, tInt)),
		field("tones", "tones", "The counts by tone or `all`.", mapOf(toneKey, tInt)),
		field("byLit", "byLit", "The slots by tone or `all`.", mapOf(toneKey, slotT)),
		field("moods", "moods", "The tones by name, or `all`.", mapOf(tString, toneKey)),
		field("small", "small", "The names by small number.", mapOf(tInt8, tString)),
		field("byBoard", "byBoard", "The counts by board.", mapOf(home, tInt)),
		field("byNode", "byNode", "The names by node.", mapOf(nodeRef, tString)),
		field("deep", "deep", "Nested maps.", mapOf(tString, mapOf(tInt, tString))),
		field("rankSlots", "rankSlots", "The slots by rank.", mapOf(ir.TypeRef{Kind: types.Enum, Named: rank}, slotT)),
		field("elemSlots", "elemSlots", "The slots by element.", mapOf(ir.TypeRef{Kind: types.Enum, Named: element}, slotT)),
	}
	board.Methods = []*ir.ExportFn{{Name: "totals", Kind: ir.FnPrecomputed, Result: mapOf(tString, tInt), Doc: "The totals by name."}, {Name: "byBoardTotals", Kind: ir.FnPrecomputed, Result: mapOf(home, tInt), Doc: "The totals by board."}, {Name: "perTone", Kind: ir.FnLookup, Result: mapOf(home, tInt), Params: []*ir.Param{{Name: "t", Type: ir.TypeRef{Kind: types.Enum, Named: tone}}}, Doc: "The counts by board, per tone."}}
	boardElem := ir.TypeRef{Kind: types.Record, Named: board}
	boards := &ir.Value{Name: "boards", Schema: mapsSchema, Doc: "The boards.", Type: ir.TypeRef{Kind: types.Table, Elem: &boardElem}}
	return &ir.Package{
		Name: mapsPkg, Dir: "demo/maps", Types: []ir.Type{element, rank, tone, slot, board}, Values: []*ir.Value{boards},
		Emits: []*ir.Emit{{Target: ir.TargetCpp, Out: "out/", Dir: "demo/maps/out", Mode: ir.ModeData, Namespace: "demo::maps"}},
	}
}
