package ir_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// CODEGEN.md §3.5, §5.2, §5.3 (DECISIONS 379, 402): the item that gets the deleted to_json and from_json pair is a baked table first, else the first type whose values are positions; "" when none (gen/cpp reserves the pair against that item).
func TestCppJSONHooksOrigin(t *testing.T) {
	codes := ir.TypeRef{Kind: types.Int, Bits: 8}
	enum := &ir.Enum{Pkg: "p", Name: "Tone", Members: []*ir.EnumMember{{Name: "soft", Wire: "soft"}}}
	coded := &ir.Enum{Pkg: "p", Name: "Coded", Codes: &codes, Members: []*ir.EnumMember{{Name: "a", Wire: "a", Code: 1}}}
	variant := &ir.Variant{Pkg: "p", Name: "Shape", Cases: []*ir.Case{{Name: "dot", Wire: "dot"}}}
	dependent := &ir.Dependent{Pkg: "p", Name: "P", Branches: []*ir.Branch{{Name: "one"}}}
	rec := &ir.Record{Pkg: "p", Name: "Item"}
	table := &ir.Value{Name: "items", Type: ir.TypeRef{Kind: types.Table, Elem: &ir.TypeRef{Kind: types.Record, Named: rec}}}
	for _, c := range []struct {
		name   string
		mode   ir.Mode
		types  []ir.Type
		values []*ir.Value
		open   []string
		want   string
	}{
		{"enum", ir.ModeTypes, []ir.Type{enum}, nil, nil, "Tone"},
		{"the first of several", ir.ModeTypes, []ir.Type{coded, variant, enum}, nil, nil, "Shape"},
		{"dependent", ir.ModeTypes, []ir.Type{dependent}, nil, nil, "P"},
		{"enum with codes", ir.ModeTypes, []ir.Type{coded}, nil, nil, ""},
		{"enum the emit opens", ir.ModeTypes, []ir.Type{enum}, nil, []string{"Tone"}, ""},
		{"a baked table before a type", ir.ModeBaked, []ir.Type{enum}, []*ir.Value{table}, nil, "items"},
		{"a table of a data emit is no id enum", ir.ModeData, nil, []*ir.Value{table}, nil, ""},
	} {
		p := &ir.Package{Name: "p", Types: c.types, Values: c.values}
		e := &ir.Emit{Target: ir.TargetCpp, Mode: c.mode, Namespace: "p", Open: c.open}
		if got := ir.PlanCppNames(p, e).JSONHooksOrigin(); got != c.want {
			t.Errorf("%s: JSONHooksOrigin = %q, want %q", c.name, got, c.want)
		}
	}
}
