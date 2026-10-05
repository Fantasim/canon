package gogen_test

import (
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// boostType is Boost, a pairs element whose ref core's hook resolves into its keyed list (WIRE.md §5.14; log-2026-10-06 "gen/go re-verify FAIL").
func (c *core) boostType() {
	c.boost, c.boostT = emberRecord(corePkg, "Boost", "", wired("rank", "rank", "", refT(corePkg, "ranks", c.rank, true)), wired("n", "n", "", intT))
}

// boostsField is LevelRange's pairs field of Boosts, slots b0/n0 and b1/n1.
func (c *core) boostsField() *ir.Field {
	return &ir.Field{Name: "boosts", Doc: "Its boosts.", Type: listT(typed(c.boost, types.Record)), Pairs: &types.Pairs{Keys: [2]string{"b{i}", "n{i}"}, Slots: 2}}
}

// marksField is LevelRange's optional map of key-only refs into core's table (CODEGEN.md §5.8: a map holds keys only).
func (c *core) marksField() *ir.Field {
	status := refT(corePkg, "statuses", c.status, false)
	return opt(wired("marks", "marks", "Its marks.", ir.TypeRef{Kind: types.Map, Key: &strT, Elem: &status}))
}

// boosted gives r one boost and one mark.
func (c *core) boosted(r *value.Record) *value.Record {
	boost := &value.Record{T: c.boostT, Fields: []value.Value{&value.Ref{Key: value.Key{S: "r2"}}, &value.Int{V: 7}}}
	r.Fields[8] = &value.Map{Keys: []value.Value{&value.Str{V: "a"}}, Vals: []value.Value{&value.Ref{Key: value.Key{S: "open"}}}}
	r.Fields[9] = &value.List{Elems: []value.Value{boost}}
	return r
}

// perk is game.world's Perk: a lookup over game.core's table that no value holds, its domain stage E's (CODEGEN.md §5.10; log-2026-10-06 "gen/go re-verify FAIL" 4).
func perk(c *core) *ir.Record {
	statuses := refT(corePkg, "statuses", c.status, false)
	ids := []value.Value{&value.Ref{Key: value.Key{S: "open"}}, &value.Ref{Key: value.Key{S: "closed"}}}
	costFor := &ir.ExportFn{
		Name: "costFor", Kind: ir.FnLookup, Result: intT, Params: []*ir.Param{{Name: "s", Type: statuses}}, Domains: [][]value.Value{ids},
	}
	rec, _ := emberRecord(worldPkg, "Perk", "Held by no value.", wired("name", "name", "", strT))
	rec.Methods = []*ir.ExportFn{costFor}
	return rec
}
