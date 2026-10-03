package wire_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/wire"
)

// formCase is one field value; site is the index of the element each finding points at, -1 for
// the value itself.
type formCase struct {
	name    string
	f       *types.Field
	v       value.Value
	sites   []int
	methods wire.Methods
}

func unitField(t types.Type, unit types.Unit) *types.Field {
	f := field("d", t)
	f.Unit = unit
	return f
}

func markerField(t types.Type, marker string) *types.Field {
	f := field("m", &types.OptionalType{Elem: t})
	f.NoneWire = []byte(marker)
	return f
}

func bitsField(t types.Type, marker string) *types.Field {
	f := field("flags", t)
	f.Enc = types.EncBits
	if marker != "" {
		f.Type, f.NoneWire = &types.OptionalType{Elem: t}, []byte(marker)
	}
	return f
}

func formCases() []formCase {
	element := codes(enum("p", "Element", "FIRE", "WATER"), 1, 2)
	element.WireCodes = true
	flag := codes(enum("p", "Flag", "tradable", "droppable", "soulbound"), 1, 2, 4)
	flags := listOf(flag)
	durs := listOf(types.DurationType)
	byName := &types.MapType{Key: types.StringType, Value: types.DurationType}
	inner := record("p", "Inner", field("ms", types.DurationType))
	empty := record("p", "Empty")
	exported := record("p", "Exported")
	exported.Methods = []*types.Method{{Name: "isFree", Export: true}}
	translated := record("p", "Translated")
	translated.Methods = []*types.Method{{Name: "twice", Export: true, Type: &types.FuncType{Params: []types.Type{types.IntType}, Result: types.IntType}}}
	free := func(*value.Record) []wire.Fn { return []wire.Fn{{Name: "isFree", Result: boolean(true)}} }
	input := unitField(types.DurationType, types.UnitS)
	input.Input = &types.Input{Env: "WAIT"}
	return []formCase{
		// WIRE.md §5.1: an integer count of the field's unit.
		{name: "whole unit", f: unitField(types.DurationType, types.UnitS), v: dur(2000)},
		{name: "fraction of the unit", f: unitField(types.DurationType, types.UnitS), v: dur(1500), sites: []int{-1}},
		{name: "fraction in a list", f: unitField(durs, types.UnitS), v: list(types.DurationType, dur(2000), dur(2500), dur(1)), sites: []int{1, 2}},
		{name: "fraction as a map value", f: unitField(byName, types.UnitM),
			v: &value.Map{T: byName, Keys: []value.Value{str("a"), str("b")}, Vals: []value.Value{dur(60000), dur(1000)}}, sites: []int{1}},
		{name: "none has no unit", f: unitField(&types.OptionalType{Elem: types.DurationType}, types.UnitS), v: none(types.DurationType)},
		{name: "a record keeps its own units", f: unitField(inner, types.UnitS), v: rec(inner, dur(1500))},
		{name: "an input field has no value", f: input, v: dur(1500)},
		// WIRE.md §5.4: a present value equal to the marker, by JSON equality.
		{name: "integer marker", f: markerField(types.IntType, "-1"), v: num(-1), sites: []int{-1}},
		{name: "other integer", f: markerField(types.IntType, "-1"), v: num(0)},
		{name: "none is the marker", f: markerField(types.IntType, "-1"), v: none(types.IntType)},
		{name: "code equal by value", f: markerField(element, "1.0"), v: member(element, 0), sites: []int{-1}},
		{name: "string marker", f: markerField(types.StringType, `""`), v: str(""), sites: []int{-1}},
		{name: "empty map marker", f: markerField(byName, "{}"), v: &value.Map{T: byName}, sites: []int{-1}},
		{name: "empty list marker", f: markerField(durs, "[]"), v: list(types.DurationType), sites: []int{-1}},
		{name: "empty record marker", f: markerField(empty, "{}"), v: rec(empty), sites: []int{-1}},
		{name: "a $ key fills the record", f: markerField(exported, "{}"), v: rec(exported), methods: free},
		{name: "a translated fn writes no $ key", f: markerField(translated, "{}"), v: rec(translated), sites: []int{-1}},
		{name: "empty bits marker", f: bitsField(flags, "0"), v: list(flag), sites: []int{-1}},
		// WIRE.md §5.3: a bits list holds each member once.
		{name: "repeated member", f: bitsField(flags, ""), v: list(flag, member(flag, 0), member(flag, 2), member(flag, 0), member(flag, 0)), sites: []int{2}},
		{name: "distinct members", f: bitsField(flags, ""), v: list(flag, member(flag, 2), member(flag, 0))},
	}
}

// WIRE.md §5.1, §5.3, §5.4 (E8102): FieldForms finds exactly the parts Encode refuses.
func TestFieldForms(t *testing.T) {
	for _, c := range formCases() {
		t.Run(c.name, func(t *testing.T) {
			forms := wire.FieldForms(c.f, c.v)
			if len(forms) != len(c.sites) {
				t.Fatalf("%d parts without a wire form, want %d", len(forms), len(c.sites))
			}
			for i, x := range forms {
				if want := siteAt(c.v, c.sites[i]); x.Site != want {
					t.Errorf("part %d points at %v, want %v", i, x.Site, want)
				}
				if x.At(source.Span{}, c.f.Name) == nil {
					t.Errorf("part %d has no finding", i)
				}
			}
			if refused := encodeField(c); refused != (len(forms) > 0) {
				t.Errorf("Encode refuses: %v, FieldForms finds %d", refused, len(forms))
			}
		})
	}
}

// siteAt is element i of a list or map value, v itself for -1.
func siteAt(v value.Value, i int) value.Value {
	switch x := v.(type) {
	case *value.List:
		if i >= 0 {
			return x.Elems[i]
		}
	case *value.Map:
		if i >= 0 {
			return x.Vals[i]
		}
	}
	return v
}

// encodeField reports whether Encode refuses a record holding c's value in c's field.
func encodeField(c formCase) bool {
	if c.f.Input != nil {
		return false
	}
	box := record("p", "Box", c.f)
	_, err := (&wire.Document{Schema: "p.Box@ae120ca0", Kind: types.Record, V: rec(box, c.v), Methods: c.methods}).Encode()
	refusals := []error{wire.ErrNotWholeUnit, wire.ErrNoneMarker, wire.ErrBitsRepeated}
	for _, r := range refusals {
		if errors.Is(err, r) {
			return true
		}
	}
	return false
}

// WIRE.md §5.1, API.md F1 (DECISIONS 283): a part's steps lead from the field's value down to it.
func TestFieldFormsSteps(t *testing.T) {
	inner := list(types.DurationType, dur(1000), dur(1500))
	outer := &value.List{T: listOf(listOf(types.DurationType)), Elems: []value.Value{inner}}
	byName := &types.MapType{Key: types.StringType, Value: listOf(types.DurationType)}
	m := &value.Map{T: byName, Keys: []value.Value{str("b")}, Vals: []value.Value{list(types.DurationType, dur(1))}}
	for _, c := range []struct {
		name string
		v    value.Value
		want []wire.Step
	}{
		{"list in a list", outer, []wire.Step{{In: outer, Index: 0}, {In: inner, Index: 1}}},
		{"list in a map", m, []wire.Step{{In: m, Key: m.Keys[0]}, {In: m.Vals[0], Index: 0}}},
	} {
		forms := wire.FieldForms(unitField(c.v.Type(), types.UnitS), c.v)
		if len(forms) != 1 || !slices.Equal(forms[0].Steps, c.want) {
			t.Errorf("%s: %+v, want one part at %+v", c.name, forms, c.want)
		}
	}
}

// WIRE.md §5.11, SPEC §9.4: a fn is stored when every parameter is finite (ir's kindOf reads it).
func TestStored(t *testing.T) {
	flag := enum("p", "Flag", "a")
	table := &types.RefType{Target: &types.Collection{}}
	keyed := &types.RefType{Target: &types.Collection{KeyedBy: field("id", types.StringType)}}
	for _, c := range []struct {
		name   string
		params []types.Type
		want   bool
	}{
		{"no parameter", nil, true},
		{"Bool, an enum, a table ref", []types.Type{types.BoolType, flag, table}, true},
		{"an Int", []types.Type{types.BoolType, types.IntType}, false},
		{"a keyed-list ref", []types.Type{keyed}, false},
		{"a dangling ref", []types.Type{&types.RefType{}}, false},
	} {
		if got := wire.Stored(&types.FuncType{Params: c.params, Result: types.IntType}); got != c.want {
			t.Errorf("%s: Stored %v, want %v", c.name, got, c.want)
		}
	}
}
