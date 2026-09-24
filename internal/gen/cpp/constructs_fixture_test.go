package cppgen_test

import (
	"math"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

const shopPkg = "demo.shop"

// The codes a vector expects (CONFORMANCE.md §3), built so no code literal sits in the source.
var (
	codeOverflow  = diag.E4101.Def().Code
	codeDivZero   = diag.E4102.Def().Code
	codeNotFinite = diag.E4104.Def().Code
	codeClamp     = diag.E4108.Def().Code
	codeWidth     = diag.E3201.Def().Code
	codeF32       = diag.E3202.Def().Code
	codeRange     = diag.E3204.Def().Code
)

func sized(bits int, signed bool) ir.TypeRef {
	return ir.TypeRef{Kind: types.Int, Bits: bits, Signed: signed}
}

var (
	tInt8    = sized(8, true)
	tInt16   = sized(16, true)
	tInt32   = sized(32, true)
	tUInt16  = sized(16, false)
	tUInt64  = sized(64, false)
	tFloat32 = ir.TypeRef{Kind: types.Float, Bits: 32}
)

// shop is the IR the constructs fixture shares between its builders.
type shop struct {
	tone, element *ir.Enum
	point, item   *ir.Record
	shelf, config *ir.Record
	node, bonus   *ir.Record
	badge         *ir.Variant
	flag, kind1   *ir.Enum
	reward        *ir.Variant
}

func (s *shop) t(n ir.Type, k types.Kind) ir.TypeRef { return ir.TypeRef{Kind: k, Named: n} }

func listOf(t ir.TypeRef) ir.TypeRef { return ir.TypeRef{Kind: types.List, Elem: &t} }

// refTo is a ref to a collection of String keys of this package.
func refTo(value string, elem ir.Type, keyed bool) ir.TypeRef {
	key := tString
	return ir.TypeRef{Kind: types.Ref, Key: &key, Ref: &ir.RefTarget{Coll: types.CollLet, Pkg: shopPkg, Value: value, Elem: elem, Keyed: keyed}}
}

// constructs is a data-mode package with every construct the generator emits: enums, a
// variant, records with every field form, a table, a keyed list, a @reload record, constants,
// stored, lookup and translated fns.
func constructs() *ir.Package {
	s := &shop{}
	s.enums()
	s.records()
	s.fields()
	p := &ir.Package{
		Name: shopPkg, Dir: "demo/shop",
		Types:  []ir.Type{s.tone, s.element, s.kind1, s.flag, s.point, s.reward, s.item, s.bonus, s.shelf, s.config, s.node, s.badge},
		Consts: s.constants(),
		Values: s.values(),
		Fns:    packageFns(s),
		Emits: []*ir.Emit{
			{Target: ir.TargetJSON, Out: "out/data/", Dir: "demo/shop/out/data"},
			{Target: ir.TargetCpp, Out: "out/cpp/", Dir: "demo/shop/out/cpp", Mode: ir.ModeData, Namespace: "demo::shop"},
		},
	}
	return p
}

func (s *shop) enums() {
	s.tone = &ir.Enum{Pkg: shopPkg, Name: "Tone", Doc: "How a message sounds.", Ordered: true, Members: []*ir.EnumMember{
		{Name: "warning", Wire: "warning", Index: 0, Doc: "Loud."},
		{Name: "info", Wire: "info", Index: 1},
		{Name: "series_1", Wire: "series-1", Index: 2},
		{Name: "legacy", Wire: "legacy", Index: 3, Retired: true},
	}}
	wide := sized(32, false)
	s.kind1 = &ir.Enum{Pkg: shopPkg, Name: "Kind1", Codes: &wide, CppDefines: "IK1_", Members: []*ir.EnumMember{
		{Name: "IK1_WEAPON", Wire: "IK1_WEAPON", Code: 1}, {Name: "GENERAL", Wire: "GENERAL", Index: 1, Code: 4},
	}}
	codes := tUInt16
	s.element = &ir.Enum{Pkg: shopPkg, Name: "Element", Codes: &codes, JSONCodes: true, Members: []*ir.EnumMember{
		{Name: "FIRE", Wire: "FIRE", Index: 0, Code: 1},
		{Name: "WATER", Wire: "WATER", Index: 1, Code: 300},
	}}
}

// nextShelf: a ref resolved in its own value, FindBy names to escape (CODEGEN.md §3.4, §5.8).
func (s *shop) nextShelf() []*ir.Field {
	next := field("next", "next", "The shelf after this one.", refTo("shelves", s.shelf, true))
	next.Optional = true
	del, i := field("delete", "delete", "", tInt), field("i", "i", "", tInt)
	del.Stable, i.Stable = true, true
	return []*ir.Field{next, del, i}
}

// alts is an optional list of refs: its getter is nullptr for none (log-2026-09-24, round 3).
func (s *shop) alts() *ir.Field {
	alts := field("alts", "alts", "", listOf(refTo("items", s.item, false)))
	alts.Optional = true
	return alts
}

// nodeRefs resolve through a boxed optional, a list and a variant case (CODEGEN.md §5.8).
func (s *shop) nodeRefs() []*ir.Field {
	hot := field("hot", "hot", "", refTo("items", s.item, false))
	hot.Optional = true
	s.badge = &ir.Variant{Pkg: shopPkg, Name: "Badge", Tag: "kind", Cases: []*ir.Case{
		{Name: "star", Wire: "star", Fields: []*ir.Field{field("of", "of", "", refTo("items", s.item, false))}},
		{Name: "plain", Wire: "plain"},
	}}
	badge := field("badge", "badge", "", s.t(s.badge, types.Variant))
	badge.Optional = true
	return []*ir.Field{hot, badge}
}

// wireFields exercise @json(pairs:), @json(bits) and a unit on a list (WIRE.md §4, §5.3, §5.14).
func (s *shop) wireFields() []*ir.Field {
	codes := sized(8, false)
	s.flag = &ir.Enum{Pkg: shopPkg, Name: "Flag", Codes: &codes, Members: []*ir.EnumMember{
		{Name: "a", Wire: "a", Code: 1}, {Name: "c", Wire: "c", Index: 1, Code: 4}, {Name: "b", Wire: "b", Index: 2, Code: 2},
	}}
	s.bonus = &ir.Record{Pkg: shopPkg, Name: "Bonus", Fields: []*ir.Field{
		field("stat", "stat", "", s.t(s.tone, types.Enum)), field("amount", "amount", "", tInt),
	}}
	bonuses := &ir.Field{Name: "bonuses", Type: listOf(s.t(s.bonus, types.Record)), Pairs: &types.Pairs{Keys: [2]string{"b{i}", "v{i}"}, Slots: 3}}
	flags := field("flags", "flags", "", listOf(s.t(s.flag, types.Enum)))
	flags.Enc = types.EncBits
	waits := field("waits", "waits", "", listOf(tDuration))
	waits.Unit = types.UnitS
	return []*ir.Field{bonuses, flags, waits}
}

func (s *shop) records() {
	s.point = &ir.Record{Pkg: shopPkg, Name: "Point", Fields: []*ir.Field{
		field("x", "x", "", tInt16), field("y", "y", "", tInt16),
	}}
	s.reward = &ir.Variant{Pkg: shopPkg, Name: "Reward", Doc: "What a quest gives.", Tag: "type", Cases: []*ir.Case{
		{Name: "item", Wire: "item", Doc: "An item, several times.", Fields: []*ir.Field{
			field("itemId", "itemId", "", tString), field("count", "count", "", tInt32),
		}},
		{Name: "gold", Wire: "gold", Fields: []*ir.Field{field("amount", "amount", "", tUInt64)}, Cpp: ir.CppCaseOptions{Name: "Coins"}},
		{Name: "nothing", Wire: "none", Doc: "No reward at all."},
	}}
	s.reward.Cases[0].Methods = []*ir.ExportFn{caseTotal()}
	s.item = &ir.Record{Pkg: shopPkg, Name: "Item", Doc: "A thing on sale.\n\nPrices are in gold."}
	s.shelf = &ir.Record{Pkg: shopPkg, Name: "Shelf"}
	s.config = &ir.Record{Pkg: shopPkg, Name: "Config", Doc: "The shop's settings."}
	s.node = &ir.Record{Pkg: shopPkg, Name: "Node", Doc: "A tree: a node holds itself."}
}

// caseTotal is Reward.item.total: `count * times`, count read from the case.
func caseTotal() *ir.ExportFn {
	body := &ir.Binary{T: tInt, Op: ir.OpMul, X: &ir.ReadRef{T: tInt32, Index: 0}, Y: &ir.ParamRef{T: tInt, Index: 0}}
	v := func(count, times, want int64, code diag.Code) *ir.Vector {
		return vec([]value.Value{num(count)}, []value.Value{num(times)}, num(want), code)
	}
	return &ir.ExportFn{
		Name: "total", Kind: ir.FnTranslated, Result: tInt, Body: body,
		Params:  []*ir.Param{{Name: "times", Type: tInt}},
		Reads:   []*ir.Read{{Name: "count", Path: []string{"count"}, Type: tInt32}},
		Vectors: []*ir.Vector{v(3, 4, 12, ""), v(3, math.MaxInt64, 0, codeOverflow), v(-2, 5, -10, "")},
	}
}

func (s *shop) fields() {
	tone, element := s.t(s.tone, types.Enum), s.t(s.element, types.Enum)
	point, reward := s.t(s.point, types.Record), s.t(s.reward, types.Variant)
	code := field("code", "code", "Stored in saves: never renumbered.", tUInt16)
	code.Stable = true
	tradable := field("tradable", "isTradable", "", tBool)
	tradable.Enc = types.EncInt
	delay := field("delay", "delaySec", "", tDuration)
	delay.Unit = types.UnitS
	note := field("note", "note", "A remark; the empty string is none on the wire.", tString)
	note.Optional, note.NoneWire = true, []byte(`""`)
	bonus := field("bonus", "bonus", "", tInt)
	bonus.Optional = true
	origin := field("origin", "origin", "", point)
	origin.Optional = true
	shelf := field("shelf", "shelf", "", refTo("shelves", s.shelf, true))
	shelf.Optional = true
	legacyMax := &ir.Field{Name: "legacyMax", WirePath: []string{"legacy", "max"}, Type: tInt}
	s.item.Fields = []*ir.Field{
		code, field("label", "label", "", tString), field("price", "price", "", tFloat),
		field("weight", "weight", "", tFloat32), tradable, field("tone", "tone", "", tone),
		field("element", "element", "", element), delay, note, bonus, origin,
		field("tags", "tags", "", listOf(tString)), field("path", "path", "", listOf(point)),
		field("reward", "reward", "", reward), shelf, legacyMax,
	}
	s.item.Methods = append(itemMethods(s), coalesceMethods()...)
	s.shelf.Fields = append([]*ir.Field{
		field("id", "id", "", tString), field("slots", "slots", "", tInt8),
		field("items", "items", "", listOf(refTo("items", s.item, false))),
	}, append(s.nextShelf(), s.wireFields()...)...)
	event := &ir.Field{Name: "event", Type: reward, Inline: true}
	scale := field("scale", "scale", "", tFloat)
	scale.Optional = true
	s.config.Fields = []*ir.Field{
		field("motd", "motd", "Message of the day.", tString), field("spawn", "spawn", "", point),
		event, field("flags", "flags", "", listOf(tone)), scale, field("tree", "tree", "", s.t(s.node, types.Record)),
		field("featured", "featured", "The item on the front page.", refTo("items", s.item, false)),
		field("picks", "picks", "", listOf(refTo("items", s.item, false))), s.alts(),
	}
	next := field("next", "next", "", s.t(s.node, types.Record))
	next.Optional = true
	s.node.Fields = append([]*ir.Field{
		field("label", "label", "", tString), next, field("kids", "kids", "", listOf(s.t(s.node, types.Record))),
	}, s.nodeRefs()...)
	s.node.Methods = []*ir.ExportFn{{
		Name: "nextLabel", Kind: ir.FnTranslated, Result: tString,
		Params: []*ir.Param{{Name: "s", Type: tString}},
		Reads:  []*ir.Read{{Name: "next_label", Path: []string{"next", "label"}, Type: tString, Optional: true}},
		Body:   &ir.Coalesce{T: tString, X: &ir.ReadRef{T: tString, Index: 0}, Y: &ir.ParamRef{T: tString, Index: 0}},
		Vectors: []*ir.Vector{
			vec([]value.Value{&value.None{}}, []value.Value{str("x")}, str("x"), ""),
			vec([]value.Value{str("second")}, []value.Value{str("x")}, str("second"), ""),
		},
	}}
}

// itemMethods: two stored fns read from `$` keys, two translated methods.
func itemMethods(s *shop) []*ir.ExportFn {
	nick := tString
	discounted := &ir.ExportFn{
		Name: "discounted", Kind: ir.FnTranslated, Result: tInt, Doc: "The price code after a discount.",
		Params: []*ir.Param{{Name: "pct", Type: tInt, Range: &types.Bound{Lo: types.Limit{I: 0}, Hi: types.Limit{I: 100}, HasLo: true, HasHi: true, HiIncluded: true}}},
		Reads:  []*ir.Read{{Name: "code", Path: []string{"code"}, Type: tUInt16}},
		Body: &ir.Binary{T: tInt, Op: ir.OpDiv, Y: lit(tInt, num(100)), X: &ir.Binary{
			T: tInt, Op: ir.OpMul,
			X: &ir.ReadRef{T: tUInt16, Index: 0},
			Y: &ir.Binary{T: tInt, Op: ir.OpSub, X: lit(tInt, num(100)), Y: &ir.ParamRef{T: tInt, Index: 0}},
		}},
	}
	d := func(code, pct, want int64, c diag.Code) *ir.Vector {
		return vec([]value.Value{num(code)}, []value.Value{num(pct)}, num(want), c)
	}
	discounted.Vectors = []*ir.Vector{d(200, 10, 180, ""), d(200, 0, 200, ""), d(200, 101, 0, codeRange), d(200, -1, 0, codeRange)}
	reward := s.t(s.reward, types.Variant)
	worth := &ir.ExportFn{
		Name: "worth", Kind: ir.FnTranslated, Result: tInt,
		Params: []*ir.Param{{Name: "times", Type: tInt}},
		Reads:  []*ir.Read{{Name: "reward", Path: []string{"reward"}, Type: reward}},
		Body: &ir.If{
			T: tInt, Cond: &ir.IsCase{T: tBool, X: &ir.ReadRef{T: reward, Index: 0}, Case: 1},
			Then: &ir.ParamRef{T: tInt, Index: 0}, Else: lit(tInt, num(0)),
		},
	}
	w := func(kind int, times, want int64) *ir.Vector {
		return vec([]value.Value{&value.CaseKind{Index: kind}}, []value.Value{num(times)}, num(want), "")
	}
	worth.Vectors = []*ir.Vector{w(0, 7, 0), w(1, 7, 7), w(2, 7, 0)}
	return append([]*ir.ExportFn{
		{Name: "heavy", Kind: ir.FnPrecomputed, Result: tBool, Doc: "Weighs more than 10."},
		{Name: "nick", Kind: ir.FnPrecomputed, Result: ir.TypeRef{Kind: types.Optional, Elem: &nick}},
		discounted, worth,
	}, lookupMethods(s)...)
}

// lookupMethods have finite parameters, read from `$` tables (CODEGEN.md §5.10, WIRE.md §5.11).
func lookupMethods(s *shop) []*ir.ExportFn {
	label, itemRef := tString, refTo("items", s.item, false)
	related := listOf(itemRef)
	return []*ir.ExportFn{
		{
			Name: "rank", Kind: ir.FnLookup, Result: tInt, Doc: "A made-up rank per tone and loudness.",
			Params: []*ir.Param{{Name: "tone", Type: s.t(s.tone, types.Enum)}, {Name: "loud", Type: tBool}},
		},
		{
			Name: "labelIn", Kind: ir.FnLookup, Result: ir.TypeRef{Kind: types.Optional, Elem: &label},
			Params: []*ir.Param{{Name: "delete", Type: s.t(s.element, types.Enum)}},
		},
		{Name: "best", Kind: ir.FnPrecomputed, Result: itemRef, Doc: "The item to buy instead."},
		{Name: "related", Kind: ir.FnPrecomputed, Result: ir.TypeRef{Kind: types.Optional, Elem: &related}},
		{
			Name: "pairFor", Kind: ir.FnLookup, Result: ir.TypeRef{Kind: types.Optional, Elem: &itemRef},
			Params: []*ir.Param{{Name: "tone", Type: s.t(s.tone, types.Enum)}},
		},
	}
}

// coalesceMethods read optional paths of self with `??` (CONFORMANCE.md §2.2–§2.3).
func coalesceMethods() []*ir.ExportFn {
	read := func(name string, t ir.TypeRef, path ...string) []*ir.Read {
		return []*ir.Read{{Name: name, Path: path, Type: t, Optional: true}}
	}
	coalesce := func(x, t ir.TypeRef) ir.PExpr {
		return &ir.Coalesce{T: t, X: &ir.ReadRef{T: x, Index: 0}, Y: &ir.ParamRef{T: t, Index: 0}}
	}
	none := &value.None{}
	pairs := func(fallback value.Value, some value.Value) []*ir.Vector {
		return []*ir.Vector{
			vec([]value.Value{none}, []value.Value{fallback}, fallback, ""),
			vec([]value.Value{some}, []value.Value{fallback}, some, ""),
		}
	}
	return []*ir.ExportFn{
		{
			Name: "bonusOr", Kind: ir.FnTranslated, Result: tInt, Body: coalesce(tInt, tInt),
			Params: []*ir.Param{{Name: "d", Type: tInt}}, Reads: read("bonus", tInt, "bonus"),
			Vectors: pairs(num(7), num(5)),
		},
		{
			Name: "originX", Kind: ir.FnTranslated, Result: tInt, Body: coalesce(tInt16, tInt),
			Params: []*ir.Param{{Name: "d", Type: tInt}}, Reads: read("origin_x", tInt16, "origin", "x"),
			Vectors: pairs(num(1), num(-3)),
		},
		{
			Name: "noteOr", Kind: ir.FnTranslated, Result: tString, Body: coalesce(tString, tString),
			Params: []*ir.Param{{Name: "s", Type: tString}}, Reads: read("note", tString, "note"),
			Vectors: pairs(str("x"), str("heavy")),
		},
	}
}

func (s *shop) constants() []*ir.Const {
	tone := s.t(s.tone, types.Enum)
	return []*ir.Const{
		{Name: "MAX_ITEMS", Doc: "The most items a shop holds.", Type: tInt, V: num(100)},
		{Name: "RATE", Type: tFloat, V: flt(0.5)},
		{Name: "GREETING", Type: tString, V: str("hi \"there\"\n")},
		{Name: "COOLDOWN", Type: tDuration, V: dur(90_000)},
		{Name: "DEFAULT_TONE", Type: tone, V: &value.Member{Index: 1}},
		{Name: "PRIMES", Type: listOf(tInt), V: &value.List{Elems: []value.Value{num(2), num(3), num(5)}}},
		{Name: "WAITS", Type: listOf(tDuration), V: &value.List{Elems: []value.Value{dur(1000), dur(2000)}}},
		{Name: "TONES", Type: listOf(tone), V: &value.List{Elems: []value.Value{&value.Member{Index: 0}, &value.Member{Index: 2}}}},
		{Name: "LIMITS", Type: ir.TypeRef{Kind: types.Map, Key: &tString, Elem: &tInt}, V: &value.Map{
			Keys: []value.Value{str("b"), str("a")}, Vals: []value.Value{num(2), num(1)},
		}},
		{Name: "delete", Type: tBool, V: boolean(true)},
	}
}

func (s *shop) values() []*ir.Value {
	item, shelf := s.t(s.item, types.Record), s.t(s.shelf, types.Record)
	return []*ir.Value{
		{Name: "items", Doc: "Everything on sale.", Reload: true, Schema: "demo.shop.Item@0000000a", Type: ir.TypeRef{Kind: types.Table, Elem: &item}},
		{
			Name: "shelves", Schema: "demo.shop.Shelf@0000000b",
			Type: ir.TypeRef{Kind: types.List, Elem: &shelf, KeyedBy: &ir.KeyField{Name: "id", WirePath: []string{"id"}}},
		},
		{Name: "config", Reload: true, Schema: "demo.shop.Config@0000000c", Type: s.t(s.config, types.Record)},
		{Name: "home", Doc: "Where the shop stands.", Schema: "demo.shop.Point@0000000d", Type: s.t(s.point, types.Record)},
	}
}
