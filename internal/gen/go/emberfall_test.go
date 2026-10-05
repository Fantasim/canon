package gogen_test

import (
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// The Emberfall shape (DECISIONS 323): game.world holds game.core's records, variant, case and dependent type in its own values, built through game.core's make hooks (CODEGEN.md §2.8, §5.14).
const (
	corePkg  = "game.core"
	worldPkg = "game.world"
)

// core is game.core, baked: an enum, a table of statuses, LevelRange (a resolved ref, a list, an optional, a stored fn and a lookup), a variant, a dependent type and a record a table of game.world holds.
type core struct {
	p                                                     *ir.Package
	tone, kind                                            *ir.Enum
	status, levels, marker, spot, region, rank, boost     *ir.Record
	reward                                                *ir.Variant
	param                                                 *ir.Dependent
	width, scaled, labelFor, rankFor, statusFor, bonusFor *ir.ExportFn
	ranksFor, rankAt                                      *ir.ExportFn
	levelsT, markerT, statusT, spotT, regionT, boostT     *types.RecordType
	rewardT                                               *types.VariantType
	coinsT                                                *types.CaseType
	toneT, kindT                                          *types.EnumType
}

// emberRecord is a record of pkg with its fields' twins.
func emberRecord(pkg, name, doc string, fields ...*ir.Field) (*ir.Record, *types.RecordType) {
	twin := &types.RecordType{Pkg: pkg, Name: name}
	for i, f := range fields {
		twin.Fields = append(twin.Fields, &types.Field{Name: f.Name, Index: i, Wire: f.Name, WirePath: f.WirePath})
	}
	return &ir.Record{Pkg: pkg, Name: name, Doc: doc, Fields: fields}, twin
}

// enumOf is an enum of pkg and its twin.
func enumOf(pkg, name string, members ...string) (*ir.Enum, *types.EnumType) {
	e := &ir.Enum{Pkg: pkg, Name: name}
	twin := &types.EnumType{Pkg: pkg, Name: name}
	for i, m := range members {
		e.Members = append(e.Members, &ir.EnumMember{Name: m, Wire: m, Index: i})
		twin.Members = append(twin.Members, &types.Member{Name: m, Wire: m, Index: i})
	}
	return e, twin
}

func newCore() *core {
	c := &core{}
	c.tone, c.toneT = enumOf(corePkg, "Tone", "soft", "loud")
	c.kind, c.kindT = enumOf(corePkg, "Kind", "num", "word")
	c.status, c.statusT = emberRecord(corePkg, "Status", "A status.", wired("label", "label", "Its label.", strT))
	ranks := c.ranks()
	c.boostType()
	c.levelRange()
	c.variant()
	c.param = &ir.Dependent{
		Pkg: corePkg, Name: "Param", Params: 1, Disc: &ir.TypeRef{Kind: types.Enum, Named: c.kind}, ByMember: []int{0, 1},
		Branches: []*ir.Branch{{Name: "num", Members: []int{0}, Type: intT}, {Name: "word", Members: []int{1}, Type: strT}},
	}
	c.marker, c.markerT = emberRecord(corePkg, "Marker", "A marker.", wired("code", "code", "Its code.", strT), wired("heal", "heal", "", intT))
	c.marker.Methods = []*ir.ExportFn{healFor()} // a translated method a row forwards (CODEGEN.md §5.9)
	c.spot, c.spotT = emberRecord(corePkg, "Spot", "", wired("x", "x", "", intT))
	spotRow := typed(c.spot, types.Record)
	c.region, c.regionT = emberRecord(corePkg, "Region", "A region: a table of core's own records.",
		wired("name", "name", "", strT), wired("spots", "spots", "", ir.TypeRef{Kind: types.Table, Elem: &spotRow}))
	statusRow := typed(c.status, types.Record)
	statuses := &ir.Value{
		Name: "statuses", Type: ir.TypeRef{Kind: types.Table, Elem: &statusRow}, IDs: []string{"open", "closed"},
		V: &value.Table{Entries: []*value.Record{c.statusEntry("open", "Open"), c.statusEntry("closed", "Closed")}},
	}
	e := goData("game/core", "core")
	e.Mode = ir.ModeBaked
	c.p = &ir.Package{
		Name: corePkg, Dir: "game/core", Types: []ir.Type{c.tone, c.kind, c.status, c.rank, c.boost, c.levels, c.reward, c.param, c.marker, c.spot, c.region},
		Values: []*ir.Value{statuses, ranks}, Emits: []*ir.Emit{e},
	}
	return c
}

// ranks is a keyed list of Rank: a ref into it is resolved by Find, and a reader refuses a key naming no entry after the hook (CODEGEN.md §2.8).
func (c *core) ranks() *ir.Value {
	var twin *types.RecordType
	c.rank, twin = emberRecord(corePkg, "Rank", "", wired("code", "code", "", strT), wired("n", "n", "", intT))
	coll := &types.Collection{Kind: types.CollLet, Pkg: corePkg, Name: "ranks"}
	rank := func(code string, n int64) value.Value {
		return &value.Record{T: twin, Fields: []value.Value{&value.Str{V: code}, &value.Int{V: n}}, Ident: &value.Identity{Coll: coll, Key: value.Key{S: code}}}
	}
	elem := typed(c.rank, types.Record)
	return &ir.Value{
		Name: "ranks", Type: ir.TypeRef{Kind: types.List, Elem: &elem, KeyedBy: &ir.KeyField{Name: "code", WirePath: []string{"code"}}},
		V: &value.List{Elems: []value.Value{rank("r1", 1), rank("r2", 2)}},
	}
}

func (c *core) statusEntry(id, label string) *value.Record {
	coll := &types.Collection{Kind: types.CollLet, Pkg: corePkg, Name: "statuses"}
	return &value.Record{T: c.statusT, Fields: []value.Value{&value.Str{V: label}}, Ident: &value.Identity{Coll: coll, Key: value.Key{S: id}}}
}

// levelRange is LevelRange: min, max, labels, tone, a ref game.core's baked hook resolves, an optional hint; width (stored) and scaled(loud) (a lookup).
func (c *core) levelRange() {
	c.levels, c.levelsT = emberRecord(corePkg, "LevelRange", "A level band.",
		wired("min", "min", "Lowest.", intT), wired("max", "max", "Highest.", intT), wired("labels", "labels", "Labels.", listT(strT)),
		wired("tone", "tone", "Its tone.", typed(c.tone, types.Enum)), wired("status", "status", "Its status.", refT(corePkg, "statuses", c.status, false)),
		opt(wired("hint", "hint", "A hint.", strT)), opt(wired("rank", "rank", "Its rank.", refT(corePkg, "ranks", c.rank, true))),
		opt(wired("alts", "alts", "Other ranks.", listT(refT(corePkg, "ranks", c.rank, true)))), c.marksField(), c.boostsField())
	loud := []*ir.Param{{Name: "loud", Type: boolT}}
	c.width = &ir.ExportFn{Name: "width", Doc: "Its width.", Kind: ir.FnPrecomputed, Result: intT}
	c.scaled = &ir.ExportFn{Name: "scaled", Doc: "Its width, doubled when loud.", Kind: ir.FnLookup, Result: intT, Params: loud}
	c.labelFor = &ir.ExportFn{Name: "labelFor", Kind: ir.FnLookup, Result: optT(strT), Params: loud}
	c.rankFor = &ir.ExportFn{Name: "rankFor", Kind: ir.FnLookup, Result: optT(refT(corePkg, "ranks", c.rank, true)), Params: loud}
	c.statusFor = &ir.ExportFn{Name: "statusFor", Kind: ir.FnLookup, Result: refT(corePkg, "statuses", c.status, false), Params: loud}
	statusParam := []*ir.Param{{Name: "s", Type: refT(corePkg, "statuses", c.status, false)}} // its domain is core's id enum (log-2026-10-06 "U2 fixes done")
	c.bonusFor = &ir.ExportFn{Name: "bonusFor", Kind: ir.FnLookup, Result: intT, Params: statusParam}
	c.ranksFor = &ir.ExportFn{Name: "ranksFor", Kind: ir.FnLookup, Result: optT(listT(refT(corePkg, "ranks", c.rank, true))), Params: statusParam}
	c.levels.Methods = []*ir.ExportFn{c.width, c.scaled, c.labelFor, c.rankFor, c.statusFor, c.bonusFor, c.ranksFor}
}

// variant is Reward: coins { amount }, nothing.
func (c *core) variant() {
	c.reward = &ir.Variant{Pkg: corePkg, Name: "Reward", Doc: "A reward.", Tag: "type", Cases: []*ir.Case{
		{Name: "coins", Wire: "coins", Doc: "Coins.", Fields: []*ir.Field{wired("amount", "amount", "How many.", intT), opt(wired("rank", "rank", "", refT(corePkg, "ranks", c.rank, true)))}},
		{Name: "nothing", Wire: "nothing"},
	}}
	// a case's lookup over core's keyed list: a reader checks its cells on the case (log-2026-10-06 "gen/go re-verify FAIL" 1)
	c.rankAt = &ir.ExportFn{Name: "rankAt", Kind: ir.FnLookup, Result: optT(refT(corePkg, "ranks", c.rank, true)), Params: []*ir.Param{{Name: "loud", Type: boolT}}}
	c.reward.Cases[0].Methods = []*ir.ExportFn{c.rankAt}
	c.rewardT = &types.VariantType{Pkg: corePkg, Name: "Reward"}
	c.coinsT = &types.CaseType{Variant: c.rewardT, Name: "coins", Wire: "coins", Index: 0, Fields: []*types.Field{{Name: "amount", Wire: "amount", WirePath: []string{"amount"}}, {Name: "rank", Index: 1, Wire: "rank", WirePath: []string{"rank"}}}}
	c.rewardT.Cases = []*types.CaseType{c.coinsT, {Variant: c.rewardT, Name: "nothing", Wire: "nothing", Index: 1}}
}

// lr is a LevelRange value with its stored results: width, and scaled's cells (CODEGEN.md §5.14: a hook takes them).
func (c *core) lr(lo, hi int64, tone int, status string) *value.Record {
	r := &value.Record{T: c.levelsT, Fields: []value.Value{
		&value.Int{V: lo}, &value.Int{V: hi}, &value.List{Elems: []value.Value{&value.Str{V: "a"}}},
		&value.Member{Index: tone}, &value.Ref{Key: value.Key{S: status}}, &value.None{}, &value.None{}, &value.None{},
		&value.None{}, &value.List{},
	}}
	w := hi - lo
	c.width.Instances = append(c.width.Instances, &ir.Instance{Recv: r, Result: &value.Int{V: w}})
	cells := func(fn *ir.ExportFn, f, t value.Value) {
		bools := []value.Value{&value.Bool{}, &value.Bool{V: true}}
		fn.Instances = append(fn.Instances, &ir.Instance{Recv: r, Table: &ir.LookupTable{Domains: [][]value.Value{bools}, Cells: []value.Value{f, t}}})
	}
	cells(c.scaled, &value.Int{V: w}, &value.Int{V: 2 * w})
	cells(c.labelFor, &value.None{}, &value.Str{V: "L"})
	cells(c.rankFor, &value.None{}, &value.Ref{Key: value.Key{S: "r1"}})
	cells(c.statusFor, &value.Ref{Key: value.Key{S: status}}, &value.Ref{Key: value.Key{S: "closed"}})
	ids := []value.Value{&value.Ref{Key: value.Key{S: "open"}}, &value.Ref{Key: value.Key{S: "closed"}}}
	c.bonusFor.Instances = append(c.bonusFor.Instances, &ir.Instance{Recv: r, Table: &ir.LookupTable{
		Domains: [][]value.Value{ids}, Cells: []value.Value{&value.Int{V: lo}, &value.Int{V: hi}},
	}})
	c.bonusFor.Domains = [][]value.Value{ids} // stage E's, whatever receivers exist (CODEGEN.md §5.10)
	c.ranksFor.Domains = [][]value.Value{ids}
	both := &value.List{Elems: []value.Value{&value.Ref{Key: value.Key{S: "r1"}}, &value.Ref{Key: value.Key{S: "r2"}}}}
	c.ranksFor.Instances = append(c.ranksFor.Instances, &ir.Instance{Recv: r, Table: &ir.LookupTable{Domains: [][]value.Value{ids}, Cells: []value.Value{&value.None{}, both}}})
	return r
}

// regionOf is a Region whose spots table holds core's own records: another package fills its rows through MakeEntry_Spot.
func (c *core) regionOf(name string) *value.Record {
	spot := func(id string, retired bool, x int64) *value.Record {
		return &value.Record{T: c.spotT, Fields: []value.Value{&value.Int{V: x}}, Ident: &value.Identity{Key: value.Key{S: id}, Retired: retired}}
	}
	spots := &value.Table{Entries: []*value.Record{spot("a", false, 1), spot("b", true, 2)}}
	return &value.Record{T: c.regionT, Fields: []value.Value{&value.Str{V: name}, spots}}
}

// coins is the case value coins { amount }.
func (c *core) coins(n int64) *value.Record {
	r := &value.Record{T: c.coinsT, Fields: []value.Value{&value.Int{V: n}, &value.None{}}}
	bools := []value.Value{&value.Bool{}, &value.Bool{V: true}}
	c.rankAt.Instances = append(c.rankAt.Instances, &ir.Instance{Recv: r, Table: &ir.LookupTable{
		Domains: [][]value.Value{bools}, Cells: []value.Value{&value.None{}, &value.Ref{Key: value.Key{S: "r2"}}},
	}})
	return r
}
