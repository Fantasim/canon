package cppgen_test

import (
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

const openPkg = "demo.open"

var tU16 = ir.TypeRef{Kind: types.Int, Bits: 16}

func mapOf(k, v ir.TypeRef) ir.TypeRef { return ir.TypeRef{Kind: types.Map, Key: &k, Elem: &v} }

// openPackage is demo.open in types mode with Dst, Stat and Rank opened: Dst has a retired member, Stat has @codes, Rank is on a numeric wire (@json(codes)) and stays closed; a record holds them as a field, an optional, a list element, a keyed-list key, a literal union and a bits list, a dependent type is decided by Dst, and four @text fns return a map, a list, a keyed list and a map of records (CODEGEN.md §5.2, §5.13, DECISIONS 362, 363).
func openPackage() *ir.Package {
	dst := &ir.Enum{Pkg: openPkg, Name: "Dst", Doc: "A destination.", Members: []*ir.EnumMember{
		{Name: "hp", Wire: "DST_HP", Doc: "Hit points."}, {Name: "mp", Wire: "DST_MP", Index: 1},
		{Name: "old", Wire: "DST_OLD", Index: 2, Retired: true},
	}}
	stat := &ir.Enum{Pkg: openPkg, Name: "Stat", Codes: &tU16, Members: []*ir.EnumMember{
		{Name: "atk", Wire: "ATK", Code: 1}, {Name: "def", Wire: "DEF", Index: 1, Code: 2}, {Name: "spd", Wire: "SPD", Index: 2, Code: 4},
	}}
	rank := &ir.Enum{Pkg: openPkg, Name: "Rank", Codes: &tU8, JSONCodes: true, Members: []*ir.EnumMember{
		{Name: "low", Wire: "low", Code: 1}, {Name: "high", Wire: "high", Index: 1, Code: 200},
	}}
	tDst, tStat, tRank := ir.TypeRef{Kind: types.Enum, Named: dst}, ir.TypeRef{Kind: types.Enum, Named: stat}, ir.TypeRef{Kind: types.Enum, Named: rank}
	entry := &ir.Record{Pkg: openPkg, Name: "Entry", Doc: "An entry.", Fields: []*ir.Field{field("dst", "dst", "", tDst), field("n", "n", "", tInt)}}
	tEntry := ir.TypeRef{Kind: types.Record, Named: entry}
	shade := &ir.Dependent{
		Pkg: openPkg, Name: "Shade", Params: 1, Disc: &tDst, ByMember: []int{0, 1, ir.NoBranch},
		Branches: []*ir.Branch{{Name: "hp", Members: []int{0}, Type: tString}, {Name: "mp", Members: []int{1}, Type: tInt}},
	}
	config := &ir.Record{Pkg: openPkg, Name: "Config", Doc: "A config.", Fields: openConfigFields(tDst, tStat, tRank, tEntry, shade)}
	return &ir.Package{
		Name: openPkg, Dir: "demo/open", Types: []ir.Type{dst, stat, rank, entry, shade, config},
		TextFns: openTextFns(tDst, tStat, tEntry),
		Emits:   []*ir.Emit{{Target: ir.TargetCpp, Dir: "out/open", Mode: ir.ModeTypes, Namespace: "demo::openenum", Open: []string{"Dst", "Stat", "Rank"}}},
	}
}

func openConfigFields(tDst, tStat, tRank, tEntry ir.TypeRef, shade *ir.Dependent) []*ir.Field {
	extra := field("extra", "extra", "", tDst)
	extra.Optional = true
	keyed := ir.TypeRef{Kind: types.List, Elem: &tEntry, KeyedBy: &ir.KeyField{Name: "dst", WirePath: []string{"dst"}}}
	mode := field("mode", "mode", "", ir.TypeRef{Kind: types.LitUnion, Elem: &tDst, Literals: []string{"all"}})
	flags := field("flags", "flags", "", listOf(tStat))
	flags.Enc = types.EncBits
	return []*ir.Field{
		field("primary", "primary", "", tDst), extra, field("list", "list", "", listOf(tDst)), field("stat", "stat", "", tStat),
		field("rank", "rank", "", tRank), field("entries", "entries", "", keyed), mode, flags,
		field("shadeOn", "shadeOn", "", tDst),
		field("shade", "shade", "", ir.TypeRef{Kind: types.TypeApp, Named: shade, Args: []*ir.Source{{From: types.ArgField, WirePath: []string{"shadeOn"}}}}),
	}
}

// openTextFns are the @text fns of demo.open: a map keyed by Dst, a list of Dst, a keyed list of Entry and a map of Stat to Entry.
func openTextFns(tDst, tStat, tEntry ir.TypeRef) []*ir.ExportFn {
	keyed := ir.TypeRef{Kind: types.List, Elem: &tEntry, KeyedBy: &ir.KeyField{Name: "dst", WirePath: []string{"dst"}}}
	return []*ir.ExportFn{
		{Name: "rates", Result: mapOf(tDst, tInt)}, {Name: "names", Result: listOf(tDst)},
		{Name: "rows", Result: keyed}, {Name: "byStat", Result: mapOf(tStat, tEntry)},
	}
}

// openUserPackage is demo.user in types mode, which imports demo.open: its record holds a Dst, a list of them and a Stat from the package that opens them, and a Fixed enum of its own that is not opened (DECISIONS 362).
func openUserPackage(base *ir.Package) *ir.Package {
	tDst := ir.TypeRef{Kind: types.Enum, Named: base.Types[0]}
	tStat := ir.TypeRef{Kind: types.Enum, Named: base.Types[1]}
	fixed := &ir.Enum{Pkg: "demo.user", Name: "Fixed", Members: []*ir.EnumMember{{Name: "low", Wire: "low"}, {Name: "high", Wire: "high", Index: 1}}}
	holder := &ir.Record{Pkg: "demo.user", Name: "Holder", Fields: []*ir.Field{
		field("dst", "dst", "", tDst), field("dsts", "dsts", "", listOf(tDst)), field("stat", "stat", "", tStat),
		field("fixed", "fixed", "", ir.TypeRef{Kind: types.Enum, Named: fixed}),
	}}
	emit := &ir.Emit{Target: ir.TargetCpp, Dir: "out/user", Mode: ir.ModeTypes, Namespace: "demo::user"}
	return &ir.Package{
		Name: "demo.user", Dir: "demo/user", Types: []ir.Type{fixed, holder}, Emits: []*ir.Emit{emit},
		Imports: []*ir.PackageRef{{Name: base.Name, Dir: base.Dir, Emits: base.Emits}},
	}
}

func openPackageFixture() *ir.Package { return openPackage() }

const refdepPkg = "demo.refdep"

// openRefdepPackage is demo.refdep in types mode, its Event opened, and its Shape too when shape is set (the table then holds opened members by address): Reward's param is a Param(event.shape) whose branch its ref's key decides through a build-time table, and Event is opened, its members declared out of wire order (CODEGEN.md §5.6, DECISIONS 350, 362).
func openRefdepPackage(shape bool) *ir.Package {
	shapes := &ir.Enum{Pkg: refdepPkg, Name: "Shape", Members: []*ir.EnumMember{
		{Name: "count", Wire: "count"}, {Name: "place", Wire: "place", Index: 1}, {Name: "none_", Wire: "none", Index: 2},
	}}
	event := &ir.Enum{Pkg: refdepPkg, Name: "Event", Members: []*ir.EnumMember{
		{Name: "reach_place", Wire: "e_reach"}, {Name: "beat_monster", Wire: "e_beat", Index: 1}, {Name: "new_day", Wire: "e_day", Index: 2},
	}}
	tShape, tEvent := ir.TypeRef{Kind: types.Enum, Named: shapes}, ir.TypeRef{Kind: types.Enum, Named: event}
	param := &ir.Dependent{
		Pkg: refdepPkg, Name: "Param", Params: 1, Disc: &tShape, ByMember: []int{0, 1, ir.NoBranch},
		Branches: []*ir.Branch{{Name: "count", Members: []int{0}, Type: tInt}, {Name: "place", Members: []int{1}, Type: tString}},
	}
	ref := ir.TypeRef{Kind: types.Ref, Key: &tEvent, Ref: &ir.RefTarget{Coll: types.CollLet, Pkg: refdepPkg, Value: "eventShapes", Keyed: true}}
	paramField := field("param", "param", "", ir.TypeRef{Kind: types.TypeApp, Named: param, Args: []*ir.Source{{From: types.ArgField, WirePath: []string{"event", "shape"}}}})
	paramField.Optional, paramField.NoneWire = true, []byte("0")
	paramField.Branches = &ir.BranchTable{Keys: []value.Key{{S: "beat_monster"}, {S: "reach_place"}}, Branches: []int{0, 1}}
	reward := &ir.Record{Pkg: refdepPkg, Name: "Reward", Fields: []*ir.Field{field("event", "event", "", ref), paramField, field("points", "points", "", tInt)}}
	return &ir.Package{
		Name: refdepPkg, Dir: "demo/refdep", Types: []ir.Type{shapes, event, param, reward},
		Emits: []*ir.Emit{{Target: ir.TargetCpp, Dir: "out/refdep", Mode: ir.ModeTypes, Namespace: "demo::refdep", Open: openList(shape)}},
	}
}

// openList is Event, and Shape too when shape is set.
func openList(shape bool) []string {
	if shape {
		return []string{"Event", "Shape"}
	}
	return []string{"Event"}
}

func openRefdepFixture() *ir.Package { return openRefdepPackage(false) }

func openRefdepShapeFixture() *ir.Package { return openRefdepPackage(true) }
