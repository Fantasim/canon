package cppgen_test

import (
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

const nestPkg = "demo.nest"

// nestRefTo is a ref to a String-keyed collection of nestPkg (log-2026-09-24 loader parity,
// nested keyed-list fields: a plain enum, a @json(codes) enum, a ref and an Int key).
func nestRefTo(value string, elem ir.Type) ir.TypeRef {
	key := tString
	return ir.TypeRef{Kind: types.Ref, Key: &key, Ref: &ir.RefTarget{Coll: types.CollLet, Pkg: nestPkg, Value: value, Elem: elem}}
}

// nestFloat32s are Holder's Float32 fields in every place a number is decoded (log-2026-09-24 A1 C++ Float32): a list element, a dotted wire name beside the same dots as a @json(path:), a variant case field and a $fn cell.
func nestFloat32s(size ir.TypeRef) (*ir.Variant, []*ir.Field, *ir.ExportFn) {
	shape := &ir.Variant{Pkg: nestPkg, Name: "Shape", Tag: "kind", Cases: []*ir.Case{
		{Name: "circle", Wire: "circle", Fields: []*ir.Field{field("r", "r", "", tFloat32)}},
		{Name: "dot", Wire: "dot"},
	}}
	fields := []*ir.Field{
		field("f32s", "f32s", "", listOf(tFloat32)),
		field("dotted", "a.b", "", tFloat32),
		{Name: "deep", WirePath: []string{"a", "b"}, Type: tFloat32},
		field("shape", "shape", "", ir.TypeRef{Kind: types.Variant, Named: shape}),
	}
	scale := &ir.ExportFn{Name: "scale", Kind: ir.FnLookup, Result: tFloat32, Params: []*ir.Param{{Name: "size", Type: size}}}
	return shape, fields, scale
}

// nestPackage is demo.nest: one record value ("value") with a keyed-list field per keyable() type (TYPES.md §9.1) but String: slots (enum), bins (@json(codes) enum), tickets (ref), items (Int, plus field b) and specials (Int via @json(path: "legacy.a")); then nestFloat32s.
func nestPackage() *ir.Package {
	size := &ir.Enum{Pkg: nestPkg, Name: "Size", Members: []*ir.EnumMember{
		{Name: "small", Wire: "small"}, {Name: "medium", Wire: "medium", Index: 1},
	}}
	codes := sized(32, false)
	grade := &ir.Enum{Pkg: nestPkg, Name: "Grade", Codes: &codes, JSONCodes: true, Members: []*ir.EnumMember{
		{Name: "a", Wire: "a", Code: 1}, {Name: "b", Wire: "b", Index: 1, Code: 2},
	}}
	tSize, tGrade := ir.TypeRef{Kind: types.Enum, Named: size}, ir.TypeRef{Kind: types.Enum, Named: grade}
	slot := &ir.Record{Pkg: nestPkg, Name: "Slot", Fields: []*ir.Field{field("size", "size", "", tSize)}}
	bin := &ir.Record{Pkg: nestPkg, Name: "Bin", Fields: []*ir.Field{field("grade", "grade", "", tGrade)}}
	item := &ir.Record{Pkg: nestPkg, Name: "Item2", Fields: []*ir.Field{field("a", "a", "", tInt), field("b", "b", "", tInt)}}
	special := &ir.Record{Pkg: nestPkg, Name: "Special", Fields: []*ir.Field{
		{Name: "legacyA", WirePath: []string{"legacy", "a"}, Type: tInt},
	}}
	widget := &ir.Record{Pkg: nestPkg, Name: "Widget", Fields: []*ir.Field{field("n", "n", "", tInt)}}
	widgetElem := ir.TypeRef{Kind: types.Record, Named: widget}
	ticket := &ir.Record{Pkg: nestPkg, Name: "Ticket", Fields: []*ir.Field{
		{Name: "label", WirePath: []string{"label"}, Type: nestRefTo("widgets", widget)},
	}}
	slotElem, binElem := ir.TypeRef{Kind: types.Record, Named: slot}, ir.TypeRef{Kind: types.Record, Named: bin}
	itemElem, ticketElem := ir.TypeRef{Kind: types.Record, Named: item}, ir.TypeRef{Kind: types.Record, Named: ticket}
	specialElem := ir.TypeRef{Kind: types.Record, Named: special}
	holder := &ir.Record{Pkg: nestPkg, Name: "Holder", Fields: []*ir.Field{
		{Name: "slots", WirePath: []string{"slots"}, Type: ir.TypeRef{Kind: types.List, Elem: &slotElem, KeyedBy: &ir.KeyField{Name: "size", WirePath: []string{"size"}}}},
		{Name: "bins", WirePath: []string{"bins"}, Type: ir.TypeRef{Kind: types.List, Elem: &binElem, KeyedBy: &ir.KeyField{Name: "grade", WirePath: []string{"grade"}}}},
		{Name: "tickets", WirePath: []string{"tickets"}, Type: ir.TypeRef{Kind: types.List, Elem: &ticketElem, KeyedBy: &ir.KeyField{Name: "label", WirePath: []string{"label"}}}},
		{Name: "items", WirePath: []string{"items"}, Type: ir.TypeRef{Kind: types.List, Elem: &itemElem, KeyedBy: &ir.KeyField{Name: "a", WirePath: []string{"a"}}}},
		{Name: "specials", WirePath: []string{"specials"}, Type: ir.TypeRef{Kind: types.List, Elem: &specialElem, KeyedBy: &ir.KeyField{Name: "legacyA", WirePath: []string{"legacy", "a"}}}},
	}}
	shape, floats, scale := nestFloat32s(tSize)
	holder.Fields = append(holder.Fields, floats...)
	holder.Methods = []*ir.ExportFn{scale}
	value := &ir.Value{Name: "value", Schema: "demo.nest.Holder@00000000", Type: ir.TypeRef{Kind: types.Record, Named: holder}}
	widgets := &ir.Value{Name: "widgets", Schema: "demo.nest.Widget@00000001", Type: ir.TypeRef{Kind: types.Table, Elem: &widgetElem}}
	return &ir.Package{
		Name: nestPkg, Dir: "demo/nest",
		Types:  []ir.Type{size, grade, slot, bin, item, special, widget, ticket, shape, holder},
		Values: []*ir.Value{value, widgets},
		Emits: []*ir.Emit{
			{Target: ir.TargetJSON, Out: "out/data/", Dir: "demo/nest/out/data"},
			{Target: ir.TargetCpp, Out: "out/cpp/", Dir: "demo/nest/out/cpp", Mode: ir.ModeData, Namespace: "demo::nest"},
		},
	}
}
