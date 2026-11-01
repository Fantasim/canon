package gogen_test

import (
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

const (
	rangeDir   = "demo/range/out/go"
	rangeGoPkg = "rangeprobe"
	itemsDir   = "demo/items/out/go"
	itemsGoPkg = "items"
)

// within is t refined by r at its level (CODEGEN.md §5.15): the generator fixtures carry no Range of their own.
func within(t ir.TypeRef, r *types.Bound) ir.TypeRef {
	t.Range = r
	return t
}

// between is the integer range lo..=hi; upTo is ..=hi; below is ..hi.
func between(lo, hi int64) *types.Bound {
	return &types.Bound{Lo: types.Limit{I: lo}, Hi: types.Limit{I: hi}, HasLo: true, HasHi: true, HiIncluded: true}
}

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

func rangeEmit(dir, pkg string, mode ir.Mode) *ir.Emit {
	return &ir.Emit{Target: ir.TargetGo, Out: "out/go/", Dir: dir, GoImport: dataModule + "/" + dir, Mode: mode, GoPackage: pkg}
}

func mapOfT(key, val ir.TypeRef) ir.TypeRef {
	return ir.TypeRef{Kind: types.Map, Key: &key, Elem: &val}
}

// rangeFlag is the Flag enum of the probe's bits list.
func rangeFlag() *ir.Enum {
	return &ir.Enum{Pkg: rangeProbePkg, Name: "Flag", Codes: &u8T, Members: []*ir.EnumMember{
		{Name: "a", Wire: "a", Code: 1}, {Name: "b", Wire: "b", Index: 1, Code: 2}, {Name: "c", Wire: "c", Index: 2, Code: 4},
	}}
}

const rangeProbePkg = "demo.range"

// rangeFields are Probe's fields, one refinement at every position CODEGEN.md §5.15 checks.
func rangeFields(sub, shape, slot ir.TypeRef, flag *ir.Enum) []*ir.Field {
	tFlag := ir.TypeRef{Kind: types.Enum, Named: flag}
	tags := within(listT(within(intT, between(0, 5))), upTo(3))
	optElem := listT(optT(within(intT, between(0, 5))))
	mapStrInt := within(mapOfT(within(strT, between(1, 3)), within(intT, between(0, 9))), upTo(2))
	mapIntStr := mapOfT(within(u8T, upTo(100)), strT)
	union := ir.TypeRef{Kind: types.LitUnion, Elem: &ir.TypeRef{Kind: types.String, Range: between(6, 8)}, Literals: []string{"auto"}}
	d := wired("d", "d", "", within(durT, &types.Bound{Lo: types.Limit{I: 1000}, Hi: types.Limit{I: 3_600_000}, HasLo: true, HasHi: true, HiIncluded: true}))
	d.Unit = types.UnitMs
	optional := func(name string, t ir.TypeRef, none string) *ir.Field {
		f := opt(wired(name, name, "", t))
		if none != "" {
			f.NoneWire = []byte(none)
		}
		return f
	}
	dfl := wired("dfl", "dfl", "", within(intT, between(1, 5)))
	dfl.Default = &value.Int{V: 9}
	flags := wired("flags", "flags", "", within(listT(tFlag), between(1, 2)))
	flags.Enc = types.EncBits
	slots := &ir.Field{Name: "slots", Type: within(listT(slot), between(1, 2)), Pairs: &types.Pairs{Keys: [2]string{"n{i}", "w{i}"}, Slots: 3}}
	return []*ir.Field{
		wired("n", "n", "", within(intT, between(1, 120))), wired("u8", "u8", "", within(u8T, between(0, 255))),
		wired("i8", "i8", "", within(i8T, between(0, 300))), wired("ex", "ex", "", within(intT, below(5))),
		wired("f", "f", "", within(fltT, floatBetween(0, 1))), wired("f32", "f32", "", within(f32T, floatUpTo(0.1))), d,
		wired("s", "s", "", within(strT, between(1, 4))), wired("tags", "tags", "", tags), wired("m", "m", "", mapStrInt),
		wired("mi", "mi", "", mapIntStr), wired("opts", "opts", "", optElem),
		optional("opt", within(intT, between(0, 5)), ""), optional("mk", within(intT, between(0, 5)), "-1"),
		wired("lit", "lit", "", union), dfl, wired("sub", "sub", "", sub), wired("shape", "shape", "", shape), flags, slots,
	}
}

// rangeExtras are Probe's dependent field after its discriminant, and a keyed list (CODEGEN.md §5.15).
func rangeExtras(flag *ir.Enum, row ir.TypeRef) (fields []*ir.Field, pick *ir.Dependent) {
	tFlag := ir.TypeRef{Kind: types.Enum, Named: flag}
	pick = &ir.Dependent{
		Pkg: rangeProbePkg, Name: "Pick", Params: 1, Disc: &tFlag, ByMember: []int{0, 1, ir.NoBranch},
		Branches: []*ir.Branch{{Name: "a", Members: []int{0}, Type: within(strT, between(2, 3))}, {Name: "b", Members: []int{1}, Type: within(intT, between(0, 5))}},
	}
	dep := wired("dep", "dep", "", ir.TypeRef{Kind: types.TypeApp, Named: pick, Args: []*ir.Source{{From: types.ArgField, WirePath: []string{"dsel"}}}})
	keyed := ir.TypeRef{Kind: types.List, Elem: &row, KeyedBy: &ir.KeyField{Name: "id", WirePath: []string{"id"}}}
	fields = []*ir.Field{wired("dsel", "dsel", "", tFlag), dep, wired("rows", "rows", "", within(keyed, upTo(2)))}
	return fields, pick
}

// rangeMaps are Probe's maps whose keys carry no refinement, for the paths under an empty key (CODEGEN.md §5.13): a scalar, a record, a list, a keyed list and a map as the value.
func rangeMaps(sub, row ir.TypeRef) []*ir.Field {
	value := within(intT, between(0, 9))
	keyed := ir.TypeRef{Kind: types.List, Elem: &row, KeyedBy: &ir.KeyField{Name: "id", WirePath: []string{"id"}}}
	return []*ir.Field{
		wired("me", "me", "", mapOfT(strT, value)), wired("mr", "mr", "", mapOfT(strT, sub)),
		wired("ml", "ml", "", mapOfT(strT, listT(within(intT, between(0, 5))))), wired("mrows", "mrows", "", mapOfT(strT, keyed)),
		wired("mm", "mm", "", mapOfT(strT, mapOfT(strT, value))),
	}
}

// rangeSeconds is Probe's Duration of 1s..=2s read in seconds: the bound is judged in milliseconds after the unit (CODEGEN.md §5.15).
func rangeSeconds() *ir.Field {
	ds := wired("ds", "ds", "", within(durT, &types.Bound{Lo: types.Limit{I: 1000}, Hi: types.Limit{I: 2000}, HasLo: true, HasHi: true, HiIncluded: true}))
	ds.Unit = types.UnitS
	return ds
}

// rangePackage is demo.range in types mode: Probe holds a refinement at every position the decoder checks, and a @text fn returns a refined list.
func rangePackage() *ir.Package {
	flag := rangeFlag()
	row := &ir.Record{Pkg: rangeProbePkg, Name: "Row", Fields: []*ir.Field{wired("id", "id", "", within(intT, between(0, 9)))}}
	subRec := &ir.Record{Pkg: rangeProbePkg, Name: "Sub", Fields: []*ir.Field{wired("v", "v", "", within(intT, between(0, 3)))}}
	slot := &ir.Record{Pkg: rangeProbePkg, Name: "Slot", Fields: []*ir.Field{wired("n", "n", "", within(intT, between(0, 3))), wired("w", "w", "", within(intT, between(0, 3)))}}
	shape := &ir.Variant{Pkg: rangeProbePkg, Name: "Shape", Tag: "kind", Cases: []*ir.Case{
		{Name: "circle", Wire: "circle", Fields: []*ir.Field{wired("r", "r", "", within(intT, between(0, 2)))}},
		{Name: "dot", Wire: "dot"},
	}}
	fields := rangeFields(typed(subRec, types.Record), typed(shape, types.Variant), typed(slot, types.Record), flag)
	extra, pick := rangeExtras(flag, typed(row, types.Record))
	extra = append(extra, rangeMaps(typed(subRec, types.Record), typed(row, types.Record))...)
	last := len(fields) - 2 // flags and slots stay last
	fields = append(fields[:last:last], append(extra, fields[last:]...)...)
	fields = append(fields, rangeSeconds())
	probe := &ir.Record{Pkg: rangeProbePkg, Name: "Probe", Fields: fields}
	caps := &ir.ExportFn{Name: "caps", Result: within(listT(within(intT, between(0, 5))), upTo(2))}
	return &ir.Package{
		Name: rangeProbePkg, Dir: "demo/range", Types: []ir.Type{flag, subRec, slot, row, shape, pick, probe},
		TextFns: []*ir.ExportFn{caps}, Emits: []*ir.Emit{rangeEmit(rangeDir, rangeGoPkg, ir.ModeTypes)},
	}
}

// rangeItems is demo.items in data mode: a keyed list of at most two items, each name 1..=3 bytes.
func rangeItems() *ir.Package {
	item := &ir.Record{Pkg: "demo.items", Name: "Item", Fields: []*ir.Field{
		wired("name", "name", "", within(strT, between(1, 3))), wired("qty", "qty", "", within(intT, between(0, 9))),
	}}
	tone := &ir.Enum{Pkg: "demo.items", Name: "Tone", Members: []*ir.EnumMember{{Name: "lo", Wire: "lo"}, {Name: "hi", Wire: "hi", Index: 1}}}
	item.Methods = []*ir.ExportFn{ // a stored `$` result and a lookup cell, each refined
		{Name: "score", Kind: ir.FnPrecomputed, Result: within(intT, between(0, 9))},
		{Name: "rank", Kind: ir.FnLookup, Result: within(intT, between(1, 6)), Params: []*ir.Param{{Name: "t", Type: typed(tone, types.Enum)}}},
	}
	elem := typed(item, types.Record)
	items := &ir.Value{Name: "items", Schema: "demo.items.Item@1", Type: within(
		ir.TypeRef{Kind: types.List, Elem: &elem, KeyedBy: &ir.KeyField{Name: "name", WirePath: []string{"name"}}}, upTo(2))}
	return &ir.Package{
		Name: "demo.items", Dir: "demo/items", Types: []ir.Type{tone, item}, Values: []*ir.Value{items},
		Emits: []*ir.Emit{rangeEmit(itemsDir, itemsGoPkg, ir.ModeData)},
	}
}
