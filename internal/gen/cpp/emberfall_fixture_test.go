package cppgen_test

import (
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// The Emberfall shape of DECISIONS 323: game.world holds game.core's classes, game.tone's through them.
const (
	tonePkg  = "game.tone"
	corePkg  = "game.core"
	worldPkg = "game.world"
)

// kindsAccessor is the @cpp(name:) of game.core's kinds: another package's reader checks keys through it (CODEGEN.md §3.5, §5.9).
var kindsAccessor = ir.NameOptions{Name: "AllKinds"}

// ember is the IR of the three packages, shared by their builders.
type ember struct {
	tone                                *ir.Enum
	status, levels, bonus, marker       *ir.Record
	reward                              *ir.Variant
	param                               *ir.Dependent
	width                               *ir.ExportFn
	statusT, levelsT, bonusT, markerT   *types.RecordType
	coins, nothing                      *types.CaseType
	zone                                *ir.Record
	zoneT                               *types.RecordType
	band                                *ir.ExportFn
	toneEmit, coreEmit                  *ir.Emit
	tTone, tLevels, tReward, tStatusRef ir.TypeRef
	levelValues                         []*value.Record
	// game.core's own tables inside its records: Kit's of its Item (entry hook), Bell's of game.tone's Chime (row hook).
	chime, item, kit, bell     *ir.Record
	chimeT, itemT, kitT, bellT *types.RecordType
	// game.core's keyed list kinds, which Gear's field, stored result and lookup cells ref: resolved by game.core's baked hook.
	kind, gear    *ir.Record
	kindT, gearT  *types.RecordType
	best, kindFor *ir.ExportFn
}

// pkgRecordType is the checker's record type of pkg's record name with the fields given.
func pkgRecordType(pkg, name string, fields ...string) *types.RecordType {
	rt := &types.RecordType{Pkg: pkg, Name: name}
	for i, f := range fields {
		rt.Fields = append(rt.Fields, &types.Field{Name: f, Index: i})
	}
	return rt
}

func newEmber() *ember {
	e := &ember{}
	e.toneEmit = &ir.Emit{Target: ir.TargetCpp, Out: "out/", Dir: "game/tone/out", Mode: ir.ModeBaked, Namespace: "game::tone"}
	e.coreEmit = &ir.Emit{Target: ir.TargetCpp, Out: "out/", Dir: "game/core/out", Mode: ir.ModeBaked, Namespace: "game::core"}
	e.tone = &ir.Enum{Pkg: tonePkg, Name: "Tone", Members: []*ir.EnumMember{{Name: "soft", Wire: "soft"}, {Name: "loud", Wire: "loud", Index: 1}}}
	e.tTone = ir.TypeRef{Kind: types.Enum, Named: e.tone}
	e.coreTypes()
	e.zoneType()
	return e
}

// coreTypes are game.core's: a table's record, a record with a resolved ref and a stored result, a variant, a dependent type, a pairs record, a row record.
func (e *ember) coreTypes() {
	e.status = &ir.Record{Pkg: corePkg, Name: "Status", Doc: "A status.", Fields: []*ir.Field{field("label", "label", "Its label.", tString)}}
	key := tString
	e.tStatusRef = ir.TypeRef{Kind: types.Ref, Key: &key, Ref: &ir.RefTarget{Coll: types.CollLet, Pkg: corePkg, Value: "statuses", Elem: e.status}}
	e.width = &ir.ExportFn{Name: "width", Kind: ir.FnPrecomputed, Result: tInt, Doc: "Its width."}
	e.levels = &ir.Record{Pkg: corePkg, Name: "LevelRange", Doc: "A level band.", Fields: []*ir.Field{
		field("min", "min", "Lowest.", tInt), field("max", "max", "Highest.", tInt),
		field("labels", "labels", "", listOf(tString)), field("tone", "tone", "", e.tTone),
		field("status", "status", "Its status, resolved by game.core's baked hook.", e.tStatusRef),
	}, Methods: []*ir.ExportFn{e.width}}
	e.tLevels = ir.TypeRef{Kind: types.Record, Named: e.levels}
	e.reward = &ir.Variant{Pkg: corePkg, Name: "Reward", Tag: "kind", Cases: []*ir.Case{
		{Name: "coins", Wire: "coins", Fields: []*ir.Field{field("amount", "amount", "", tInt)}}, {Name: "nothing", Wire: "nothing"},
	}}
	e.tReward = ir.TypeRef{Kind: types.Variant, Named: e.reward}
	// type Param(t: Tone) = match t { soft => String, loud => Int } (CODEGEN.md §5.6).
	e.param = &ir.Dependent{Pkg: corePkg, Name: "Param", Params: 1, Disc: &e.tTone, ByMember: []int{0, 1},
		Branches: []*ir.Branch{{Name: "soft", Members: []int{0}, Type: tString}, {Name: "loud", Members: []int{1}, Type: tInt}}}
	e.bonus = &ir.Record{Pkg: corePkg, Name: "StatBonus", Fields: []*ir.Field{field("stat", "stat", "", tString), field("value", "value", "", tInt)}}
	e.marker = &ir.Record{Pkg: corePkg, Name: "Marker", Fields: []*ir.Field{field("code", "code", "", tString)}}
	next := field("next", "next", "Boxed: a row's base assigned through a hook holds a std::unique_ptr.", ir.TypeRef{Kind: types.Record, Named: e.marker})
	next.Optional = true
	e.marker.Fields = append(e.marker.Fields, next)
	e.statusT, e.levelsT = pkgRecordType(corePkg, "Status", "label"), pkgRecordType(corePkg, "LevelRange", "min", "max", "labels", "tone", "status")
	e.bonusT, e.markerT = pkgRecordType(corePkg, "StatBonus", "stat", "value"), pkgRecordType(corePkg, "Marker", "code", "next")
	e.chime = &ir.Record{Pkg: tonePkg, Name: "Chime", Fields: []*ir.Field{field("pitch", "pitch", "", tInt)}}
	e.item = &ir.Record{Pkg: corePkg, Name: "Item", Fields: []*ir.Field{field("name", "name", "", tString)}}
	itemElem, chimeElem := ir.TypeRef{Kind: types.Record, Named: e.item}, ir.TypeRef{Kind: types.Record, Named: e.chime}
	e.kit = &ir.Record{Pkg: corePkg, Name: "Kit", Fields: []*ir.Field{field("items", "items", "", ir.TypeRef{Kind: types.Table, Elem: &itemElem})}}
	e.bell = &ir.Record{Pkg: corePkg, Name: "Bell", Fields: []*ir.Field{field("chimes", "chimes", "", ir.TypeRef{Kind: types.Table, Elem: &chimeElem})}}
	e.gearTypes()
	e.chimeT, e.itemT = pkgRecordType(tonePkg, "Chime", "pitch"), pkgRecordType(corePkg, "Item", "name")
	e.kitT, e.bellT = pkgRecordType(corePkg, "Kit", "items"), pkgRecordType(corePkg, "Bell", "chimes")
	e.coins = &types.CaseType{Name: "coins", Index: 0, Fields: []*types.Field{{Name: "amount"}}}
	e.nothing = &types.CaseType{Name: "nothing", Index: 1}
}

// gearTypes are game.core's Kind, the element of its keyed list kinds, and Gear, whose field, precomputed result and lookup cells ref kinds.
func (e *ember) gearTypes() {
	e.kind = &ir.Record{Pkg: corePkg, Name: "Kind", Fields: []*ir.Field{field("code", "code", "", tString), field("label", "label", "", tString)}}
	key := tString
	kindRef := ir.TypeRef{Kind: types.Ref, Key: &key, Ref: &ir.RefTarget{Coll: types.CollLet, Pkg: corePkg, Value: "kinds", Elem: e.kind, Keyed: true, Cpp: kindsAccessor}}
	optKind := ir.TypeRef{Kind: types.Optional, Elem: &kindRef}
	e.best = &ir.ExportFn{Name: "best", Kind: ir.FnPrecomputed, Result: kindRef}
	e.kindFor = &ir.ExportFn{Name: "kindFor", Kind: ir.FnLookup, Result: optKind, Params: []*ir.Param{{Name: "t", Type: e.tTone}}}
	e.gear = &ir.Record{Pkg: corePkg, Name: "Gear", Fields: []*ir.Field{field("kind", "kind", "", kindRef)}, Methods: []*ir.ExportFn{e.best, e.kindFor}}
	e.kindT, e.gearT = pkgRecordType(corePkg, "Kind", "code", "label"), pkgRecordType(corePkg, "Gear", "kind")
	e.bonus.Fields[0] = field("stat", "stat", "A key into kinds: a pairs slot's resolved ref.", kindRef)
}

// gearOf is a Gear value with its stored results.
func (e *ember) gearOf(kind, best string, cells ...value.Value) *value.Record {
	r := &value.Record{T: e.gearT, Fields: []value.Value{key(kind)}}
	e.best.Instances = append(e.best.Instances, &ir.Instance{Recv: r, Result: key(best)})
	tones := [][]value.Value{{member(0), member(1)}}
	e.kindFor.Instances = append(e.kindFor.Instances, &ir.Instance{Recv: r, Table: &ir.LookupTable{Domains: tones, Cells: cells}})
	return r
}

// zoneType is game.world's record holding every shape: a field, a list element, a map value, a variant, a dependent type, a table field, a pairs field, a stored result.
func (e *ember) zoneType() {
	param := field("param", "param", "", ir.TypeRef{Kind: types.TypeApp, Named: e.param, Args: []*ir.Source{{From: types.ArgField, WirePath: []string{"tone"}}}})
	markerElem := ir.TypeRef{Kind: types.Record, Named: e.marker}
	bonuses := field("bonuses", "bonuses", "", listOf(ir.TypeRef{Kind: types.Record, Named: e.bonus}))
	bonuses.WirePath, bonuses.Pairs = nil, &types.Pairs{Keys: [2]string{"stat{i}", "value{i}"}, Slots: 2}
	key := tString
	e.band = &ir.ExportFn{Name: "band", Kind: ir.FnPrecomputed, Result: e.tLevels, Doc: "Its band."}
	e.zone = &ir.Record{Pkg: worldPkg, Name: "Zone", Doc: "A zone.", Fields: []*ir.Field{
		field("levels", "levels", "Its levels.", e.tLevels), field("trail", "trail", "", listOf(e.tLevels)),
		field("byName", "byName", "", ir.TypeRef{Kind: types.Map, Key: &key, Elem: &e.tLevels}),
		field("reward", "reward", "", e.tReward), field("tone", "tone", "", e.tTone), param,
		field("markers", "markers", "", ir.TypeRef{Kind: types.Table, Elem: &markerElem}), bonuses,
		field("kit", "kit", "", ir.TypeRef{Kind: types.Record, Named: e.kit}), field("bell", "bell", "", ir.TypeRef{Kind: types.Record, Named: e.bell}),
		field("gear", "gear", "", ir.TypeRef{Kind: types.Record, Named: e.gear}),
	}, Methods: []*ir.ExportFn{e.band}}
	e.zoneT = pkgRecordType(worldPkg, "Zone", "levels", "trail", "byName", "reward", "tone", "param", "markers", "bonuses", "kit", "bell", "gear")
}

// emberTone is game.tone, baked.
func emberTone() *ir.Package {
	e := newEmber()
	return &ir.Package{Name: tonePkg, Dir: "game/tone", Types: []ir.Type{e.tone, e.chime}, Emits: []*ir.Emit{e.toneEmit}}
}

// emberCore is game.core, baked, with its statuses table.
func emberCore() *ir.Package { return newEmber().core() }

// emberCoreFor is game.core as a world of mode m reads it: without LevelRange's stored width for a types-mode reader (E8014, CODEGEN.md §5.13).
func emberCoreFor(m ir.Mode) *ir.Package {
	e := newEmber()
	if m == ir.ModeTypes {
		e.levels.Methods = nil
	}
	return e.core()
}

// emberCamp is game.camp, a data emit into game.world's namespace that reads game.core's LevelRange too: both readers live in one binary (log-2026-10-06 "U3 review FAIL").
func emberCamp() *ir.Package {
	e := newEmber()
	camp := &ir.Record{Pkg: "game.camp", Name: "Camp", Fields: []*ir.Field{field("levels", "levels", "", e.tLevels)}}
	return &ir.Package{
		Name: "game.camp", Dir: "game/camp", Types: []ir.Type{camp},
		Values:  []*ir.Value{{Name: "camp", Schema: "game.camp.Camp@00000003", Type: ir.TypeRef{Kind: types.Record, Named: camp}}},
		Emits:   []*ir.Emit{{Target: ir.TargetCpp, Out: "out/", Dir: "game/camp/out", Mode: ir.ModeData, Namespace: "game::world"}},
		Imports: []*ir.PackageRef{{Name: corePkg, Dir: "game/core", Emits: []*ir.Emit{e.coreEmit}}, {Name: tonePkg, Dir: "game/tone", Emits: []*ir.Emit{e.toneEmit}}},
	}
}

func (e *ember) core() *ir.Package {
	elem := ir.TypeRef{Kind: types.Record, Named: e.status}
	coll := &types.Collection{Kind: types.CollLet, Pkg: corePkg, Name: "statuses"}
	entry := func(id, label string) *value.Record {
		return &value.Record{T: e.statusT, Fields: []value.Value{str(label)}, Ident: &value.Identity{Coll: coll, Key: value.Key{S: id}}}
	}
	kindElem, kinds := ir.TypeRef{Kind: types.Record, Named: e.kind}, &types.Collection{Kind: types.CollLet, Pkg: corePkg, Name: "kinds"}
	kindOf := func(code, label string) value.Value {
		return &value.Record{T: e.kindT, Fields: []value.Value{str(code), str(label)}, Ident: &value.Identity{Coll: kinds, Key: value.Key{S: code}}}
	}
	kindList := &ir.Value{Name: "kinds", Doc: "The kinds.", Cpp: kindsAccessor, Type: ir.TypeRef{Kind: types.List, Elem: &kindElem, KeyedBy: &ir.KeyField{Name: "code", WirePath: []string{"code"}}},
		V: list(kindOf("a", "Axe"), kindOf("b", "Bow"))}
	statuses := &ir.Value{Name: "statuses", Doc: "The statuses.", IDs: []string{"open", "closed"}, Type: ir.TypeRef{Kind: types.Table, Elem: &elem},
		V: &value.Table{Entries: []*value.Record{entry("open", "Open"), entry("closed", "Closed")}}}
	return &ir.Package{
		Name: corePkg, Dir: "game/core", Types: []ir.Type{e.status, e.levels, e.reward, e.param, e.bonus, e.marker, e.item, e.kit, e.bell, e.kind, e.gear},
		Values: []*ir.Value{statuses, kindList}, Emits: []*ir.Emit{e.coreEmit},
		Imports: []*ir.PackageRef{{Name: tonePkg, Dir: "game/tone", Emits: []*ir.Emit{e.toneEmit}}},
	}
}

// world is game.world in mode m: a table of its own records, a table of game.core's (rows), a root value of game.core's, a constant of game.tone's enum; baked, a constexpr lookup over game.core's table ids.
func (e *ember) world(m ir.Mode) *ir.Package {
	p := &ir.Package{
		Name: worldPkg, Dir: "game/world", Types: []ir.Type{e.zone},
		Consts: []*ir.Const{{Name: "START_TONE", Doc: "The first tone.", Type: e.tTone, V: member(1)}},
		Emits:  []*ir.Emit{{Target: ir.TargetCpp, Out: "out/", Dir: "game/world/out", Mode: m, Namespace: "game::world"}},
		Imports: []*ir.PackageRef{
			{Name: corePkg, Dir: "game/core", Emits: []*ir.Emit{e.coreEmit}}, {Name: tonePkg, Dir: "game/tone", Emits: []*ir.Emit{e.toneEmit}},
		},
	}
	if m == ir.ModeTypes { // no stored result, no map, no Gear and its stored results (E8014, CODEGEN.md §5.13)
		fields := e.zone.Fields[:len(e.zone.Fields)-1]
		e.zone.Methods, e.zone.Fields = nil, append(fields[:2], fields[3:]...)
		e.levels.Methods = nil
		return p
	}
	p.Values = e.worldValues()
	if m == ir.ModeBaked {
		p.Fns = []*ir.ExportFn{{Name: "bandWidth", Doc: "A status's band width.", Kind: ir.FnLookup, Result: tInt,
			Params:  []*ir.Param{{Name: "s", Type: e.tStatusRef}},
			Table:   &ir.LookupTable{Domains: [][]value.Value{{key("open"), key("closed")}}, Cells: []value.Value{num(1), num(2)}},
			Domains: [][]value.Value{{key("open"), key("closed")}}}}
		p.Types = append(p.Types, e.patrol())
		p.Fns = append(p.Fns, &ir.ExportFn{Name: "emptyWidth", Doc: "Over a table with no entry: a constexpr empty domain.", Kind: ir.FnLookup, Result: tInt,
			Params: []*ir.Param{{Name: "s", Type: e.tStatusRef}}, Table: &ir.LookupTable{Domains: [][]value.Value{{}}}, Domains: [][]value.Value{{}}})
	}
	e.instances()
	return p
}

func emberWorld(m ir.Mode) func() *ir.Package {
	return func() *ir.Package { return newEmber().world(m) }
}

// levelsOf is a LevelRange value; its width is precomputed.
func (e *ember) levelsOf(lo, hi int64, tone int, status string, labels ...string) *value.Record {
	ls := make([]value.Value, len(labels))
	for i, l := range labels {
		ls[i] = str(l)
	}
	r := &value.Record{T: e.levelsT, Fields: []value.Value{num(lo), num(hi), list(ls...), member(tone), key(status)}}
	e.levelValues = append(e.levelValues, r)
	return r
}

func (e *ember) worldValues() []*ir.Value {
	zoneElem, levelsElem := ir.TypeRef{Kind: types.Record, Named: e.zone}, e.tLevels
	markerColl := &types.Collection{Kind: types.CollField, Pkg: worldPkg, Name: "markers"}
	markers := &value.Table{Entries: []*value.Record{
		{T: e.markerT, Fields: []value.Value{str("g"), &value.Record{T: e.markerT, Fields: []value.Value{str("n"), &value.None{}}}}, Ident: &value.Identity{Coll: markerColl, Key: value.Key{S: "gate"}}},
		{T: e.markerT, Fields: []value.Value{str("w"), &value.None{}}, Ident: &value.Identity{Coll: markerColl, Key: value.Key{S: "well"}, Retired: true}},
	}}
	bonuses := list(&value.Record{T: e.bonusT, Fields: []value.Value{key("a"), num(2)}}, &value.Record{T: e.bonusT, Fields: []value.Value{key("b"), num(1)}})
	forest := &value.Record{T: e.zoneT, Ident: &value.Identity{Coll: &types.Collection{Kind: types.CollLet, Pkg: worldPkg, Name: "zones"}, Key: value.Key{S: "forest"}},
		Fields: []value.Value{
			e.levelsOf(1, 5, 0, "open", "a", "b"), list(e.levelsOf(2, 4, 1, "closed")),
			&value.Map{Keys: []value.Value{str("east")}, Vals: []value.Value{e.levelsOf(3, 9, 0, "open")}},
			&value.Record{T: e.coins, Fields: []value.Value{num(3)}}, member(1), num(7), markers, bonuses, e.kitValue(), e.bellValue(),
			e.gearOf("a", "b", key("b"), &value.None{}),
		}}
	e.band.Instances = []*ir.Instance{{Recv: forest, Result: e.levelsOf(10, 20, 1, "closed")}}
	bandsColl := &types.Collection{Kind: types.CollLet, Pkg: worldPkg, Name: "bands"}
	low, high := e.levelsOf(1, 3, 0, "open"), e.levelsOf(4, 8, 1, "closed")
	low.Ident, high.Ident = &value.Identity{Coll: bandsColl, Key: value.Key{S: "low"}}, &value.Identity{Coll: bandsColl, Key: value.Key{S: "high"}, Retired: true}
	return []*ir.Value{
		{Name: "zones", Doc: "The zones.", Schema: "game.world.Zone@00000001", IDs: []string{"forest"}, Type: ir.TypeRef{Kind: types.Table, Elem: &zoneElem}, V: &value.Table{Entries: []*value.Record{forest}}},
		{Name: "bands", Doc: "Level bands.", Schema: "game.core.LevelRange@00000002", IDs: []string{"low", "high"}, Type: ir.TypeRef{Kind: types.Table, Elem: &levelsElem}, V: &value.Table{Entries: []*value.Record{low, high}}},
		{Name: "start", Doc: "The start band.", Schema: "game.core.LevelRange@00000002", Type: e.tLevels, V: e.levelsOf(1, 2, 1, "open")},
	}
}

// instances are game.core's precomputed widths of every LevelRange value game.world holds.
func (e *ember) instances() {
	for _, r := range e.levelValues {
		lo, hi := r.Fields[0].(*value.Int).V, r.Fields[1].(*value.Int).V
		e.width.Instances = append(e.width.Instances, &ir.Instance{Recv: r, Result: num(hi - lo)})
	}
}

// nestedEntry is entry id of a table field of a game.core record.
func nestedEntry(t *types.RecordType, field, id string, retired bool, fields ...value.Value) *value.Record {
	coll := &types.Collection{Kind: types.CollField, Pkg: corePkg, Name: field}
	return &value.Record{T: t, Fields: fields, Ident: &value.Identity{Coll: coll, Key: value.Key{S: id}, Retired: retired}}
}

// kitValue is a Kit: game.core's own table of its Item, whose rows another package builds through game.core's entry hook.
func (e *ember) kitValue() *value.Record {
	items := &value.Table{Entries: []*value.Record{nestedEntry(e.itemT, "items", "sword", false, str("Sword"))}}
	return &value.Record{T: e.kitT, Fields: []value.Value{items}}
}

// bellValue is a Bell: game.core's table of game.tone's Chime, whose rows are game.core's ChimeRow, built through game.core's row hook.
func (e *ember) bellValue() *value.Record {
	chimes := &value.Table{Entries: []*value.Record{nestedEntry(e.chimeT, "chimes", "c1", true, num(440))}}
	return &value.Record{T: e.bellT, Fields: []value.Value{chimes}}
}

// patrol is a record of game.world no value holds: its lookup over game.core's statuses has no receiver, yet its domain is the owner table's ids, which stage E enumerates (CODEGEN.md §5.10; log-2026-10-06 "U5 review FAIL" 2).
func (e *ember) patrol() *ir.Record {
	return &ir.Record{Pkg: worldPkg, Name: "Patrol", Fields: []*ir.Field{field("n", "n", "", tInt)}, Methods: []*ir.ExportFn{
		{Name: "widthAt", Kind: ir.FnLookup, Result: tInt, Params: []*ir.Param{{Name: "s", Type: e.tStatusRef}},
			Domains: [][]value.Value{{key("open"), key("closed")}}},
	}}
}
