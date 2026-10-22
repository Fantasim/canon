package cppgen_test

import (
	"fmt"
	"math"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

const helpersPkg = "demo.helpers"

var tI16 = ir.TypeRef{Kind: types.Int, Bits: 16, Signed: true}

// members are enum members named and wired by pairs, with the given codes (nil: none), in declaration order.
func members(pairs []string, codes []int64) []*ir.EnumMember {
	var out []*ir.EnumMember
	for i := 0; i+1 < len(pairs); i += 2 {
		m := &ir.EnumMember{Name: pairs[i], Wire: pairs[i+1], Index: i / 2}
		if codes != nil {
			m.Code = codes[i/2]
		}
		out = append(out, m)
	}
	return out
}

// helpersPackage is demo.helpers in types mode: every kind of enum whose helpers the .gen.cpp defines as tables (DECISIONS 383, CODEGEN.md §5.2): a plain enum with a retired member and a UTF-8 wire whose byte >= 0x80 sorts after the "z" of another, @codes enums whose codes are not in declaration order (unsigned and signed), an @codes(Int) enum holding INT64_MIN and INT64_MAX, two members with equal wires, zero-member enums with and without @codes, a variant's kind enum, and two opened enums, one with @codes, an equal wire and a UTF-8 wire, one without members.
func helpersPackage() *ir.Package {
	tone := &ir.Enum{Pkg: helpersPkg, Name: "Tone", Members: members([]string{"warning", "warning", "info", "info", "series_1", "s\u00e9rie-1", "old", "sz"}, nil)}
	tone.Members[3].Retired = true
	element := &ir.Enum{Pkg: helpersPkg, Name: "Element", Codes: &tU8, Members: members([]string{"FIRE", "FIRE", "WATER", "WATER", "EARTH", "EARTH"}, []int64{3, 1, 200})}
	element.Members[1].Retired = true // DECISIONS 395
	delta := &ir.Enum{Pkg: helpersPkg, Name: "Delta", Codes: &tI16, Members: members([]string{"up", "up", "down", "down", "flat", "flat"}, []int64{7, -5, 0})}
	extreme := &ir.Enum{Pkg: helpersPkg, Name: "Extreme", Codes: &tInt, Members: members([]string{"LOW", "LOW", "MID", "MID", "HIGH", "HIGH"}, []int64{math.MinInt64, 0, math.MaxInt64})}
	dup := &ir.Enum{Pkg: helpersPkg, Name: "Dup", Members: members([]string{"a", "x", "b", "x", "c", "w"}, nil)}
	nothing := &ir.Enum{Pkg: helpersPkg, Name: "Nothing"}
	none := &ir.Enum{Pkg: helpersPkg, Name: "NoCode", Codes: &tU8}
	stat := &ir.Enum{Pkg: helpersPkg, Name: "Stat", Codes: &tU16, Members: members([]string{"atk", "ATK", "def", "SPA", "spd", "SP\u00c9", "alias", "ATK"}, []int64{4, 1, 2, 9})}
	stat.Members[1].Retired = true // DECISIONS 395
	blank := &ir.Enum{Pkg: helpersPkg, Name: "Blank"}
	openDup := &ir.Enum{Pkg: helpersPkg, Name: "OpenDup", Members: members([]string{"first", "x", "second", "x", "third", "third", "fourth", "y", "fifth", "y"}, nil)}
	openDup.Members[1].Retired, openDup.Members[3].Retired = true, true // DECISIONS 383, 395: the first declared of equal wires answers
	shape := &ir.Variant{Pkg: helpersPkg, Name: "Shape", Tag: "kind", Cases: []*ir.Case{
		{Name: "circle", Wire: "circle", Fields: []*ir.Field{field("r", "r", "", tInt)}}, {Name: "dot", Wire: "DOT"},
	}}
	shape.Cases[1].Retired = true // DECISIONS 395
	return &ir.Package{
		Name: helpersPkg, Dir: "demo/helpers", Types: []ir.Type{tone, element, delta, extreme, dup, nothing, none, stat, blank, openDup, shape},
		Emits: []*ir.Emit{{Target: ir.TargetCpp, Dir: "out/helpers", Mode: ir.ModeTypes, Namespace: "demo::helpers", Open: []string{"Stat", "Blank", "OpenDup"}}},
	}
}

// bigCodedPackage is demo.big in baked mode with one @codes(UInt32) enum of n members, its codes descending, for the size test (DECISIONS 383).
func bigCodedPackage(n int) *ir.Package {
	e := &ir.Enum{Pkg: "demo.big", Name: "Item", Codes: &ir.TypeRef{Kind: types.Int, Bits: 32}}
	for i := range n {
		name := fmt.Sprintf("ITEM_%05d", i)
		e.Members = append(e.Members, &ir.EnumMember{Name: name, Wire: name, Index: i, Code: int64(3*(n-i) + 1)})
	}
	return &ir.Package{
		Name: "demo.big", Dir: "demo/big", Types: []ir.Type{e},
		Emits: []*ir.Emit{{Target: ir.TargetCpp, Dir: "out/big", Mode: ir.ModeBaked, Namespace: "demo::big"}},
	}
}
