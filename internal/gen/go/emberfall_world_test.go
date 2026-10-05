package gogen_test

import (
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// ember is game.world over game.core: a record Zone holding core's values in every position a reader or a literal builds (CODEGEN.md §2.8, §5.9, §5.14).
type ember struct {
	c     *core
	p     *ir.Package
	zone  *ir.Record
	zoneT *types.RecordType
	best  *ir.ExportFn
	baked bool
}

// emberfall is game.core (baked) and game.world in mode: Zone's fields hold a LevelRange, a list and a map of them, the variant, the enum and its dependent type, a table of Markers (rows of world's own row type), and baked a case-typed field; a stored fn returns a LevelRange.
func emberfall(mode ir.Mode) (*core, *ir.Package) {
	e := &ember{c: newCore(), baked: mode == ir.ModeBaked}
	e.zoneType()
	emit := goData("game/world", "world")
	emit.Mode = mode
	e.p = &ir.Package{
		Name: worldPkg, Dir: "game/world", Types: []ir.Type{e.zone},
		Imports: []*ir.PackageRef{{Name: corePkg, Dir: "game/core", Emits: e.c.p.Emits}}, Emits: []*ir.Emit{emit},
	}
	e.values()
	if e.baked {
		e.bakedOnly()
	}
	return e.c, e.p
}

func (e *ember) zoneType() {
	c := e.c
	levels := typed(c.levels, types.Record)
	markerRow := typed(c.marker, types.Record)
	fields := []*ir.Field{
		wired("levels", "levels", "Its levels.", levels), wired("bands", "bands", "", listT(levels)),
		wired("named", "named", "", ir.TypeRef{Kind: types.Map, Key: &strT, Elem: &levels}),
		wired("reward", "reward", "Its reward.", typed(c.reward, types.Variant)), wired("kind", "kind", "", typed(c.kind, types.Enum)),
		wired("param", "param", "", ir.TypeRef{Kind: types.TypeApp, Named: c.param, Args: []*ir.Source{{From: types.ArgField, WirePath: []string{"kind"}}}}),
		wired("markers", "markers", "Its markers.", ir.TypeRef{Kind: types.Table, Elem: &markerRow}),
		wired("region", "region", "", typed(c.region, types.Record)),
	}
	if e.baked {
		fields = append(fields, wired("coins", "coins", "A case-typed field.", ir.TypeRef{Kind: types.Case, Named: c.reward, Case: c.reward.Cases[0]}))
	}
	e.zone, e.zoneT = emberRecord(worldPkg, "Zone", "A zone.", fields...)
	e.best = &ir.ExportFn{Name: "best", Doc: "Its best band.", Kind: ir.FnPrecomputed, Result: levels}
	e.zone.Methods = []*ir.ExportFn{e.best}
}

// values are zones (a table of Zone), bands (a table of core's LevelRange: world's rows) and start (a LevelRange value).
func (e *ember) values() {
	c := e.c
	zoneRow, levels := typed(e.zone, types.Record), typed(c.levels, types.Record)
	zones := &ir.Value{Name: "zones", Doc: "The zones.", Schema: "game.world.Zone@00000001", Type: ir.TypeRef{Kind: types.Table, Elem: &zoneRow}, IDs: []string{"forest"}}
	bands := &ir.Value{Name: "bands", Doc: "Level bands.", Schema: "game.core.LevelRange@00000002", Type: ir.TypeRef{Kind: types.Table, Elem: &levels}, IDs: []string{"low", "high"}}
	start := &ir.Value{Name: "start", Doc: "The start band.", Schema: "game.core.LevelRange@00000003", Type: levels}
	if e.baked {
		zones.V = &value.Table{Entries: []*value.Record{e.forest()}}
		bands.V = &value.Table{Entries: []*value.Record{e.entry("bands", "low", c.lr(1, 5, 0, "open"), false), e.entry("bands", "high", c.lr(6, 9, 1, "closed"), true)}}
		begin := c.lr(1, 2, 1, "open")
		begin.Fields[6] = &value.Ref{Key: value.Key{S: "r2"}} // a ref core's hook finds in its keyed list
		begin.Fields[7] = &value.List{Elems: []value.Value{&value.Ref{Key: value.Key{S: "r1"}}, &value.Ref{Key: value.Key{S: "r2"}}}}
		start.V = c.boosted(begin)
	}
	e.p.Values = []*ir.Value{zones, bands, start}
	preset := listT(levels)
	e.p.Consts = []*ir.Const{{Name: "PRESET", Doc: "Preset bands.", Type: preset, V: &value.List{Elems: []value.Value{c.lr(3, 4, 0, "closed")}}}}
}

// entry makes r a row of world's table value.
func (e *ember) entry(table, id string, r *value.Record, retired bool) *value.Record {
	r.Ident = &value.Identity{Coll: &types.Collection{Kind: types.CollLet, Pkg: worldPkg, Name: table}, Key: value.Key{S: id}, Retired: retired}
	return r
}

// forest is the one zone, with its stored best band.
func (e *ember) forest() *value.Record {
	c := e.c
	gate := &value.Record{T: c.markerT, Fields: []value.Value{&value.Str{V: "g"}, &value.Int{V: 5}}, Ident: &value.Identity{Key: value.Key{S: "gate"}}}
	fields := []value.Value{
		c.lr(1, 5, 0, "open"), &value.List{Elems: []value.Value{c.lr(2, 3, 1, "closed")}},
		&value.Map{Keys: []value.Value{&value.Str{V: "north"}}, Vals: []value.Value{c.lr(4, 6, 0, "open")}},
		c.coins(3), &value.Member{Index: 1}, &value.Str{V: "wind"},
		&value.Table{Entries: []*value.Record{gate}}, c.regionOf("r"), c.coins(7),
	}
	r := e.entry("zones", "forest", &value.Record{T: e.zoneT, Fields: fields}, false)
	e.best.Instances = []*ir.Instance{{Recv: r, Result: c.lr(9, 12, 1, "closed")}}
	return r
}

// bakedOnly is levelOf(s: ref core.statuses), a lookup over another package's table, which its id enum indexes (CODEGEN.md §5.10).
func (e *ember) bakedOnly() {
	e.p.Types = append(e.p.Types, perk(e.c)) // data mode refuses a lookup over a table (E8013)
	status := refT(corePkg, "statuses", e.c.status, false)
	keys := []value.Value{&value.Ref{Key: value.Key{S: "open"}}, &value.Ref{Key: value.Key{S: "closed"}}}
	e.p.Fns = []*ir.ExportFn{{
		Name: "levelOf", Doc: "A level per status.", Kind: ir.FnLookup, Result: intT, Params: []*ir.Param{{Name: "s", Type: status}},
		Table:   &ir.LookupTable{Domains: [][]value.Value{keys}, Cells: []value.Value{&value.Int{V: 10}, &value.Int{V: 20}}},
		Domains: [][]value.Value{keys},
	}}
}
