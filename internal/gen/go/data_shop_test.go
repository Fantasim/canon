package gogen_test

import (
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// shop is a data-mode package with every construct data mode reads: enums with and without
// @codes, a variant, every field form, a table, a keyed list, @reload records and a record
// value, refs resolved in a value and in the snapshot, stored and lookup fns, and a baked import.
type shop struct {
	base                                           *ir.Package
	role                                           *ir.Enum
	rank                                           *ir.Record
	tone, element, flag                            *ir.Enum
	point, item, shelf, bonus, config, node, gauge *ir.Record
	link                                           *ir.Record
	reward, badge                                  *ir.Variant
}

func newShop() *ir.Package {
	s := &shop{base: base()}
	s.role, s.rank = s.base.Types[0].(*ir.Enum), s.base.Types[1].(*ir.Record)
	s.enums()
	s.records()
	s.itemFields()
	s.shelfFields()
	s.configFields()
	p := &ir.Package{
		Name: shopPkg, Dir: "demo/shop",
		Imports: []*ir.PackageRef{{Name: basePkg, Dir: "demo/base", Emits: s.base.Emits}},
		Types:   []ir.Type{s.tone, s.element, s.flag, s.point, s.reward, s.item, s.bonus, s.link, s.shelf, s.config, s.node, s.badge, s.gauge},
		Emits:   []*ir.Emit{goData("demo/shop", "shop")},
	}
	p.Values = s.values()
	return p
}

func (s *shop) enums() {
	s.tone = &ir.Enum{Pkg: shopPkg, Name: "Tone", Doc: "How a message sounds.", Ordered: true, Members: []*ir.EnumMember{
		{Name: "warning", Wire: "warning", Doc: "Loud."}, {Name: "info", Wire: "info", Index: 1},
		{Name: "series_1", Wire: "series-1", Index: 2}, {Name: "legacy", Wire: "legacy", Index: 3, Retired: true},
	}}
	s.element = &ir.Enum{Pkg: shopPkg, Name: "Element", Codes: &u16T, JSONCodes: true, Members: []*ir.EnumMember{
		{Name: "FIRE", Wire: "FIRE", Code: 1}, {Name: "WATER", Wire: "WATER", Index: 1, Code: 300},
	}}
	s.flag = &ir.Enum{Pkg: shopPkg, Name: "Flag", Codes: &u8T, Members: []*ir.EnumMember{
		{Name: "a", Wire: "a", Code: 1}, {Name: "c", Wire: "c", Index: 1, Code: 4}, {Name: "b", Wire: "b", Index: 2, Code: 2},
	}}
}

func (s *shop) records() {
	s.point = &ir.Record{Pkg: shopPkg, Name: "Point", Fields: []*ir.Field{wired("x", "x", "", i16T), wired("y", "y", "", i16T)}}
	s.reward = &ir.Variant{Pkg: shopPkg, Name: "Reward", Doc: "What a quest gives.", Tag: "type", Cases: []*ir.Case{
		{Name: "item", Wire: "item", Doc: "An item, several times.", Fields: []*ir.Field{wired("itemId", "itemId", "", strT), wired("count", "count", "", i32T)}},
		{Name: "gold", Wire: "gold", Fields: []*ir.Field{wired("amount", "amount", "", u64T)}},
		{Name: "nothing", Wire: "none", Doc: "No reward at all."},
	}}
	s.item = &ir.Record{Pkg: shopPkg, Name: "Item", Doc: "A thing on sale.\n\nPrices are in gold."}
	s.shelf = &ir.Record{Pkg: shopPkg, Name: "Shelf"}
	s.bonus = &ir.Record{Pkg: shopPkg, Name: "Bonus", Fields: []*ir.Field{wired("stat", "stat", "", typed(s.tone, types.Enum)), wired("amount", "amount", "", intT)}}
	s.config = &ir.Record{Pkg: shopPkg, Name: "Config", Doc: "The shop's settings."}
	s.node = &ir.Record{Pkg: shopPkg, Name: "Node", Doc: "A tree: a node holds itself."}
	s.gauge = &ir.Record{Pkg: shopPkg, Name: "Gauge", Doc: "Held by no value: never decoded.", Fields: []*ir.Field{wired("level", "level", "", intT)}}
}

func (s *shop) itemFields() {
	code := wired("code", "code", "Stored in saves: never renumbered.", u16T)
	code.Stable = true
	tradable := wired("tradable", "isTradable", "", boolT)
	tradable.Enc = types.EncInt
	delay := wired("delay", "delaySec", "", durT)
	delay.Unit = types.UnitS
	note := opt(wired("note", "note", "A remark; the empty string is none on the wire.", strT))
	note.NoneWire = []byte(`""`)
	s.item.Fields = []*ir.Field{
		code, wired("label", "label", "", strT), wired("price", "price", "", fltT), wired("weight", "weight", "", f32T),
		tradable, wired("tone", "tone", "", typed(s.tone, types.Enum)), wired("element", "element", "", typed(s.element, types.Enum)),
		delay, note, opt(wired("bonus", "bonus", "", intT)), opt(wired("origin", "origin", "", typed(s.point, types.Record))),
		wired("tags", "tags", "", listT(strT)), wired("path", "path", "", listT(typed(s.point, types.Record))),
		wired("reward", "reward", "", typed(s.reward, types.Variant)),
		opt(wired("shelf", "shelf", "", refT(shopPkg, "shelves", s.shelf, true))),
		{Name: "legacyMax", WirePath: []string{"legacy", "max"}, Type: intT},
		wired("role", "role", "", typed(s.role, types.Enum)), wired("rank", "rank", "", refT(basePkg, "ranks", s.rank, false)),
		wired("odd", "odd,key", "A key a struct tag cannot hold.", strT),
	}
	s.item.Methods = s.itemMethods()
}

func (s *shop) itemMethods() []*ir.ExportFn {
	items := refT(shopPkg, "items", s.item, false)
	shelves := refT(shopPkg, "shelves", s.shelf, true)
	return []*ir.ExportFn{
		{Name: "heavy", Kind: ir.FnPrecomputed, Result: boolT, Doc: "Weighs more than 10."},
		{Name: "nick", Kind: ir.FnPrecomputed, Result: optT(strT)},
		{Name: "best", Kind: ir.FnPrecomputed, Result: items, Doc: "The item to buy instead."},
		{Name: "related", Kind: ir.FnPrecomputed, Result: optT(listT(items))},
		{
			Name: "score", Kind: ir.FnLookup, Result: intT, Doc: "A made-up score per tone and loudness.",
			Params: []*ir.Param{{Name: "tone", Type: typed(s.tone, types.Enum)}, {Name: "loud", Type: boolT}},
		},
		{Name: "labelIn", Kind: ir.FnLookup, Result: optT(strT), Params: []*ir.Param{{Name: "delete", Type: typed(s.element, types.Enum)}}},
		{Name: "shelfFor", Kind: ir.FnLookup, Result: optT(shelves), Params: []*ir.Param{{Name: "tone", Type: typed(s.tone, types.Enum)}}},
		{Name: "pairFor", Kind: ir.FnLookup, Result: optT(items), Params: []*ir.Param{{Name: "tone", Type: typed(s.tone, types.Enum)}}},
		{Name: "kin", Kind: ir.FnLookup, Result: listT(items), Params: []*ir.Param{{Name: "loud", Type: boolT}}},
	}
}

func (s *shop) shelfFields() {
	del, i := wired("delete", "delete", "", intT), wired("i", "i", "", intT)
	next := opt(wired("next", "next", "The shelf after this one.", refT(shopPkg, "shelves", s.shelf, true)))
	bonuses := &ir.Field{Name: "bonuses", Type: listT(typed(s.bonus, types.Record)), Pairs: &types.Pairs{Keys: [2]string{"b{i}", "v{i}"}, Slots: 3}}
	flags := wired("flags", "flags", "", listT(typed(s.flag, types.Enum)))
	flags.Enc = types.EncBits
	waits := wired("waits", "waits", "", listT(durT))
	waits.Unit = types.UnitS
	lit := wired("lit", "lit", "", boolT)
	lit.Enc = types.EncInt
	s.link = &ir.Record{Pkg: shopPkg, Name: "Link", Fields: []*ir.Field{
		wired("to", "to", "", refT(shopPkg, "shelves", s.shelf, true)), wired("note", "note", "", intT),
	}}
	links := &ir.Field{Name: "links", Type: listT(typed(s.link, types.Record)), Pairs: &types.Pairs{Keys: [2]string{"l{i}", "n{i}"}, Slots: 2}}
	s.shelf.Fields = []*ir.Field{
		wired("id", "id", "", strT), wired("slots", "slots", "", i8T),
		wired("items", "items", "", listT(refT(shopPkg, "items", s.item, false))), next, del, i, bonuses, flags, waits,
		lit, {Name: "depth", WirePath: []string{"size", "depth"}, Type: intT}, links,
	}
}

func (s *shop) configFields() {
	items := refT(shopPkg, "items", s.item, false)
	s.badge = &ir.Variant{Pkg: shopPkg, Name: "Badge", Tag: "kind", Cases: []*ir.Case{
		{Name: "star", Wire: "star", Fields: []*ir.Field{wired("of", "of", "", items)}},
		{Name: "plain", Wire: "plain"},
	}}
	s.config.Fields = []*ir.Field{
		wired("motd", "motd", "Message of the day.", strT), wired("spawn", "spawn", "", typed(s.point, types.Record)),
		{Name: "event", Type: typed(s.reward, types.Variant), Inline: true},
		wired("flags", "flags", "", listT(typed(s.tone, types.Enum))), opt(wired("scale", "scale", "", fltT)),
		wired("tree", "tree", "", typed(s.node, types.Record)),
		wired("featured", "featured", "The item on the front page.", items),
		wired("picks", "picks", "", listT(items)), opt(wired("alts", "alts", "", listT(items))),
	}
	s.node.Fields = []*ir.Field{
		wired("label", "label", "", strT), opt(wired("next", "next", "", typed(s.node, types.Record))),
		wired("kids", "kids", "", listT(typed(s.node, types.Record))),
		opt(wired("hot", "hot", "", items)), opt(wired("badge", "badge", "", typed(s.badge, types.Variant))),
	}
}

func (s *shop) values() []*ir.Value {
	item, shelf := typed(s.item, types.Record), typed(s.shelf, types.Record)
	return []*ir.Value{
		{Name: "items", Doc: "Everything on sale.", Reload: true, Schema: schema["items"], Type: ir.TypeRef{Kind: types.Table, Elem: &item}},
		{
			Name: "shelves", Doc: "Where items stand.", Schema: schema["shelves"],
			Type: ir.TypeRef{Kind: types.List, Elem: &shelf, KeyedBy: &ir.KeyField{Name: "id", WirePath: []string{"id"}}},
		},
		{Name: "config", Reload: true, Schema: schema["config"], Type: typed(s.config, types.Record)},
		{Name: "home", Doc: "Where the shop stands.", Schema: schema["home"], Type: typed(s.point, types.Record)},
	}
}
