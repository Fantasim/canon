package ir_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// TestGoPlanEnumTables is CODEGEN.md §5.2 (DECISIONS 430, D-ENUMTAB): the plan reserves exactly the tables gen/go writes. A closed enum, kind enum, id and branch enum have a by-wire table and no keys or found table; a dense @codes enum has no codes table; a wires table only when a wire differs from its name; an opened enum has one keys table, members (and, with codes that are not dense, by-code) indexes, and names only when a name differs from its wire.
func TestGoPlanEnumTables(t *testing.T) {
	codes := ir.TypeRef{Kind: types.Int, Bits: 8}
	member := func(name, wire string, code int64) *ir.EnumMember {
		return &ir.EnumMember{Name: name, Wire: wire, Code: code}
	}
	enum := func(name string, withCodes bool, members ...*ir.EnumMember) *ir.Enum {
		e := &ir.Enum{Pkg: "p", Name: name, Members: members}
		if withCodes {
			e.Codes = &codes
		}
		return e
	}
	p := &ir.Package{Name: "p", Types: []ir.Type{
		enum("Dense", true, member("a", "a", 5), member("b", "b", 6)),
		enum("Sparse", true, member("a", "a", 9), member("b", "b", 2)),
		enum("Wired", false, member("a", "x", 0)),
		enum("ODense", true, member("a", "a", 5), member("b", "b", 6)),
		enum("OSparse", true, member("a", "a", 9), member("b", "b", 2)),
		enum("OWired", false, member("a", "x", 0)),
		enum("OSame", false, member("a", "a", 0)),
		&ir.Variant{Pkg: "p", Name: "V", Cases: []*ir.Case{{Name: "c", Wire: "w"}}},
	}}
	emit := &ir.Emit{Target: ir.TargetGo, Mode: ir.ModeTypes, Open: []string{"ODense", "OSparse", "OWired", "OSame"}}
	pkg := ir.GoScopeNames(ir.PlanGoNames(p, emit))["package"]
	for _, c := range []struct {
		enum string
		want []string
		not  []string
	}{
		{"Dense", []string{"enumNames_Dense", "enumByWire_Dense", "enumMembers_Dense"}, []string{"enumCodes_Dense", "enumKeys_Dense", "enumFound_Dense", "enumWires_Dense"}},
		{"Sparse", []string{"enumNames_Sparse", "enumByWire_Sparse", "enumCodes_Sparse", "enumMembers_Sparse"}, []string{"enumKeys_Sparse", "enumFound_Sparse", "enumWires_Sparse"}},
		{"Wired", []string{"enumNames_Wired", "enumWires_Wired", "enumByWire_Wired"}, []string{"enumKeys_Wired", "enumFound_Wired"}},
		{"ODense", []string{"enumKeys_ODense", "enumMembers_ODense", "enumNumbers_ODense"}, []string{"enumNames_ODense", "enumCodes_ODense", "enumByCode_ODense", "enumByWire_ODense"}},
		{"OSparse", []string{"enumKeys_OSparse", "enumMembers_OSparse", "enumNumbers_OSparse", "enumCodes_OSparse", "enumByCode_OSparse"}, []string{"enumNames_OSparse"}},
		{"OWired", []string{"enumKeys_OWired", "enumNames_OWired", "enumMembers_OWired"}, []string{"enumByWire_OWired", "enumFound_OWired"}},
		{"OSame", []string{"enumKeys_OSame", "enumMembers_OSame"}, []string{"enumNames_OSame"}},
		{"VKind", []string{"enumNames_VKind", "enumWires_VKind", "enumByWire_VKind"}, []string{"enumKeys_VKind", "enumFound_VKind"}},
	} {
		for _, name := range c.want {
			if !pkg[name] {
				t.Errorf("%s: the plan does not reserve %s", c.enum, name)
			}
		}
		for _, name := range c.not {
			if pkg[name] {
				t.Errorf("%s: the plan reserves %s", c.enum, name)
			}
		}
	}
	if ir.GoTableByWire != "enumByWire" {
		t.Errorf("GoTableByWire = %q", ir.GoTableByWire)
	}
}
