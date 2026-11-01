package cppgen_test

import (
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

const rangePkg = "demo.range"

// within is t refined by r at its level (CODEGEN.md §5.15): the generator fixtures carry no Range of their own.
func within(t ir.TypeRef, r *types.Bound) ir.TypeRef {
	t.Range = r
	return t
}

// upTo is the integer range ..=hi; below is ..hi.
func upTo(hi int64) *types.Bound {
	return &types.Bound{Hi: types.Limit{I: hi}, HasHi: true, HiIncluded: true}
}

func below(hi int64) *types.Bound { return &types.Bound{Hi: types.Limit{I: hi}, HasHi: true} }

// floatUpTo is the float range ..=hi; floatBetween is lo..hi.
func floatUpTo(hi float64) *types.Bound {
	return &types.Bound{Hi: types.Limit{F: hi}, HasHi: true, HiIncluded: true}
}

func floatBetween(lo, hi float64) *types.Bound {
	return &types.Bound{Lo: types.Limit{F: lo}, Hi: types.Limit{F: hi}, HasLo: true, HasHi: true}
}

// rangeFlag is the Flag enum of the probe's bits list.
func rangeFlag() *ir.Enum {
	return &ir.Enum{Pkg: rangePkg, Name: "Flag", Codes: &tU8, Members: []*ir.EnumMember{
		{Name: "a", Wire: "a", Code: 1}, {Name: "b", Wire: "b", Index: 1, Code: 2}, {Name: "c", Wire: "c", Index: 2, Code: 4},
	}}
}

// rangeFields are Probe's fields, one refinement at every position CODEGEN.md §5.15 checks.
func rangeFields(sub, shape, slot ir.TypeRef, flag *ir.Enum) []*ir.Field {
	tStr, tFlag := within(tString, bound(1, 4)), ir.TypeRef{Kind: types.Enum, Named: flag}
	tags := within(listOf(within(tInt, bound(0, 5))), upTo(3))
	opt := ir.TypeRef{Kind: types.Optional, Elem: &ir.TypeRef{}}
	*opt.Elem = within(tInt, bound(0, 5))
	optElem := listOf(opt)
	key, val := within(tString, bound(1, 3)), within(tInt, bound(0, 9))
	mapStrInt := within(mapOf(key, val), upTo(2))
	mapIntStr := mapOf(within(tU8, upTo(100)), tString)
	union := ir.TypeRef{Kind: types.LitUnion, Elem: &ir.TypeRef{Kind: types.String, Range: bound(6, 8)}, Literals: []string{"auto"}}
	d := field("d", "d", "", within(tDuration, &types.Bound{Lo: types.Limit{I: 1000}, Hi: types.Limit{I: 3_600_000}, HasLo: true, HasHi: true, HiIncluded: true}))
	d.Unit = types.UnitMs
	optional := func(name string, t ir.TypeRef, none string) *ir.Field {
		f := field(name, name, "", t)
		f.Optional = true
		if none != "" {
			f.NoneWire = []byte(none)
		}
		return f
	}
	dfl := field("dfl", "dfl", "", within(tInt, bound(1, 5)))
	dfl.Default = num(9)
	flags := field("flags", "flags", "", within(listOf(tFlag), bound(1, 2)))
	flags.Enc = types.EncBits
	slots := &ir.Field{Name: "slots", Type: within(listOf(slot), bound(1, 2)), Pairs: &types.Pairs{Keys: [2]string{"n{i}", "w{i}"}, Slots: 3}}
	return []*ir.Field{
		field("n", "n", "", within(tInt, bound(1, 120))), field("u8", "u8", "", within(tU8, bound(0, 255))),
		field("i8", "i8", "", within(tInt8, bound(0, 300))), field("ex", "ex", "", within(tInt, below(5))),
		field("f", "f", "", within(tFloat, floatBetween(0, 1))), field("f32", "f32", "", within(tFloat32, floatUpTo(0.1))), d,
		field("s", "s", "", tStr), field("tags", "tags", "", tags), field("m", "m", "", mapStrInt), field("mi", "mi", "", mapIntStr),
		field("opts", "opts", "", optElem), optional("opt", within(tInt, bound(0, 5)), ""), optional("mk", within(tInt, bound(0, 5)), "-1"),
		field("lit", "lit", "", union), dfl, field("sub", "sub", "", sub), field("shape", "shape", "", shape), flags, slots,
	}
}

// rangeExtras are Probe's dependent field after its discriminant, and a keyed list (CODEGEN.md §5.15).
func rangeExtras(flag *ir.Enum, row ir.TypeRef) (fields []*ir.Field, pick *ir.Dependent) {
	tFlag := ir.TypeRef{Kind: types.Enum, Named: flag}
	pick = &ir.Dependent{
		Pkg: rangePkg, Name: "Pick", Params: 1, Disc: &tFlag, ByMember: []int{0, 1, ir.NoBranch},
		Branches: []*ir.Branch{{Name: "a", Members: []int{0}, Type: within(tString, bound(2, 3))}, {Name: "b", Members: []int{1}, Type: within(tInt, bound(0, 5))}},
	}
	dep := field("dep", "dep", "", ir.TypeRef{Kind: types.TypeApp, Named: pick, Args: []*ir.Source{{From: types.ArgField, WirePath: []string{"dsel"}}}})
	keyed := ir.TypeRef{Kind: types.List, Elem: &row, KeyedBy: &ir.KeyField{Name: "id", WirePath: []string{"id"}}}
	fields = []*ir.Field{field("dsel", "dsel", "", tFlag), dep, field("rows", "rows", "", within(keyed, upTo(2)))}
	return fields, pick
}

// rangeMaps are Probe's maps whose keys carry no refinement, for the paths under an empty key (CODEGEN.md §5.13): a scalar, a record, a list, a keyed list and a map as the value.
func rangeMaps(sub, row ir.TypeRef) []*ir.Field {
	value := within(tInt, bound(0, 9))
	keyed := ir.TypeRef{Kind: types.List, Elem: &row, KeyedBy: &ir.KeyField{Name: "id", WirePath: []string{"id"}}}
	return []*ir.Field{
		field("me", "me", "", mapOf(tString, value)), field("mr", "mr", "", mapOf(tString, sub)),
		field("ml", "ml", "", mapOf(tString, listOf(within(tInt, bound(0, 5))))), field("mrows", "mrows", "", mapOf(tString, keyed)),
		field("mm", "mm", "", mapOf(tString, mapOf(tString, value))),
	}
}

// rangePackage is demo.range in types mode: Probe holds a refinement at every position the decoder checks, and a @text fn returns a refined list.
func rangePackage() *ir.Package {
	flag := rangeFlag()
	row := &ir.Record{Pkg: rangePkg, Name: "Row", Fields: []*ir.Field{field("id", "id", "", within(tInt, bound(0, 9)))}}
	subRec := &ir.Record{Pkg: rangePkg, Name: "Sub", Fields: []*ir.Field{field("v", "v", "", within(tInt, bound(0, 3)))}}
	slot := &ir.Record{Pkg: rangePkg, Name: "Slot", Fields: []*ir.Field{field("n", "n", "", within(tInt, bound(0, 3))), field("w", "w", "", within(tInt, bound(0, 3)))}}
	shape := &ir.Variant{Pkg: rangePkg, Name: "Shape", Tag: "kind", Cases: []*ir.Case{
		{Name: "circle", Wire: "circle", Fields: []*ir.Field{field("r", "r", "", within(tInt, bound(0, 2)))}},
		{Name: "dot", Wire: "dot"},
	}}
	fields := rangeFields(
		ir.TypeRef{Kind: types.Record, Named: subRec}, ir.TypeRef{Kind: types.Variant, Named: shape}, ir.TypeRef{Kind: types.Record, Named: slot}, flag)
	extra, pick := rangeExtras(flag, ir.TypeRef{Kind: types.Record, Named: row})
	extra = append(extra, rangeMaps(ir.TypeRef{Kind: types.Record, Named: subRec}, ir.TypeRef{Kind: types.Record, Named: row})...)
	last := len(fields) - 2 // flags and slots stay last
	fields = append(fields[:last:last], append(extra, fields[last:]...)...)
	probe := &ir.Record{Pkg: rangePkg, Name: "Probe", Fields: fields}
	emit := &ir.Emit{Target: ir.TargetCpp, Out: "out/", Dir: "demo/range/out", Mode: ir.ModeTypes, Namespace: "demo::range"}
	caps := &ir.ExportFn{Name: "caps", Result: within(listOf(within(tInt, bound(0, 5))), upTo(2))}
	return &ir.Package{
		Name: rangePkg, Dir: "demo/range", Types: []ir.Type{flag, subRec, slot, row, shape, pick, probe},
		TextFns: []*ir.ExportFn{caps}, Emits: []*ir.Emit{emit},
	}
}

// rangeItems is demo.items in data mode: a keyed list of at most two items, each name 1..=3 bytes.
func rangeItems() *ir.Package {
	item := &ir.Record{Pkg: "demo.items", Name: "Item", Fields: []*ir.Field{
		field("name", "name", "", within(tString, bound(1, 3))), field("qty", "qty", "", within(tInt, bound(0, 9))),
	}}
	tone := &ir.Enum{Pkg: "demo.items", Name: "Tone", Members: []*ir.EnumMember{{Name: "lo", Wire: "lo"}, {Name: "hi", Wire: "hi", Index: 1}}}
	item.Methods = []*ir.ExportFn{ // a stored `$` result and a lookup cell, each refined
		{Name: "score", Kind: ir.FnPrecomputed, Result: within(tInt, bound(0, 9))},
		{Name: "rank", Kind: ir.FnLookup, Result: within(tInt, bound(1, 6)), Params: []*ir.Param{{Name: "t", Type: ir.TypeRef{Kind: types.Enum, Named: tone}}}},
	}
	elem := ir.TypeRef{Kind: types.Record, Named: item}
	items := &ir.Value{Name: "items", Schema: "demo.items.Item@1", Type: within(
		ir.TypeRef{Kind: types.List, Elem: &elem, KeyedBy: &ir.KeyField{Name: "name", WirePath: []string{"name"}}}, upTo(2))}
	emit := &ir.Emit{Target: ir.TargetCpp, Out: "out/", Dir: "demo/items/out", Mode: ir.ModeData, Namespace: "demo::items"}
	return &ir.Package{Name: "demo.items", Dir: "demo/items", Types: []ir.Type{tone, item}, Values: []*ir.Value{items}, Emits: []*ir.Emit{emit}}
}
