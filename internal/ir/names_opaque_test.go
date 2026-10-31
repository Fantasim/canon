package ir_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// TestGoPlanReservesBranchEnum is CODEGEN.md §5.6 (DECISIONS 402): a dependent type's branch enum is an opaque struct, so the plan reserves its Parse<B>, its names, keys and found tables in the package and String, MarshalText, UnmarshalText, Position and the field position in its own scope.
func TestGoPlanReservesBranchEnum(t *testing.T) {
	p := &ir.Package{Name: "p", Types: []ir.Type{&ir.Dependent{Pkg: "p", Name: "P", Branches: []*ir.Branch{{Name: "one"}}}}}
	scopes := ir.GoScopeNames(ir.PlanGoNames(p, &ir.Emit{Target: ir.TargetGo, Mode: ir.ModeBaked}))
	for scope, names := range map[string][]string{ //canon:unordered each scope is checked alone
		"package": {"ParsePBranch", "enumNames_PBranch", "enumKeys_PBranch", "enumFound_PBranch"},
		"PBranch": {"String", "MarshalText", "UnmarshalText", "Position", "position"},
	} {
		for _, name := range names {
			if !scopes[scope][name] {
				t.Errorf("%s does not reserve %s", scope, name)
			}
		}
	}
	for _, name := range []string{"Wire", "Retired", "Members"} {
		if scopes["PBranch"][name] {
			t.Errorf("PBranch reserves %s, which a branch enum has not", name)
		}
	}
}

// TestCppPlanReservesJSONHooksForPositions is CODEGEN.md §3.5, §5.2 (DECISIONS 402): a package of a types-mode emit with an enum without @codes, a variant or a dependent type reserves to_json and from_json in its namespace, as a baked table does; an enum with @codes alone does not.
func TestCppPlanReservesJSONHooksForPositions(t *testing.T) {
	codes := ir.TypeRef{Kind: types.Int, Bits: 8}
	enum := &ir.Enum{Pkg: "p", Name: "Tone", Members: []*ir.EnumMember{{Name: "soft", Wire: "soft"}}}
	coded := &ir.Enum{Pkg: "p", Name: "Coded", Codes: &codes, Members: []*ir.EnumMember{{Name: "a", Wire: "a", Code: 1}}}
	for _, c := range []struct {
		name  string
		types []ir.Type
		open  []string
		want  bool
	}{
		{"enum", []ir.Type{enum}, nil, true},
		{"variant", []ir.Type{&ir.Variant{Pkg: "p", Name: "Shape", Cases: []*ir.Case{{Name: "dot", Wire: "dot"}}}}, nil, true},
		{"dependent", []ir.Type{&ir.Dependent{Pkg: "p", Name: "P", Branches: []*ir.Branch{{Name: "one"}}}}, nil, true},
		{"enum with codes", []ir.Type{coded}, nil, false},
		{"enum the emit opens", []ir.Type{enum}, []string{"Tone"}, false},
	} {
		p := &ir.Package{Name: "p", Types: c.types}
		e := &ir.Emit{Target: ir.TargetCpp, Mode: ir.ModeTypes, Namespace: "p", Open: c.open}
		names := ir.CppScopeNames(ir.PlanCppNames(p, e))["p"]
		for _, hook := range []string{"to_json", "from_json"} {
			if names[hook] != c.want {
				t.Errorf("%s: %s reserved %v, want %v", c.name, hook, names[hook], c.want)
			}
		}
	}
}

// TestTSPlanReservesForeignEnumIndex is CODEGEN.md §5.2 (DECISIONS 402): a ts file that reaches an enum of another package, whose package has a ts emit, reserves <alias>_<E>Index; an owner with no ts emit reserves nothing.
func TestTSPlanReservesForeignEnumIndex(t *testing.T) {
	const want = "base_core_ToneIndex"
	tone := &ir.Enum{Pkg: "base.core", Name: "Tone", Members: []*ir.EnumMember{{Name: "soft", Wire: "soft"}}}
	if got := ir.TSForeignEnumIndexName(tone); got != want {
		t.Fatalf("TSForeignEnumIndexName = %q, want %q", got, want)
	}
	field := &ir.Field{Name: "tone", Type: ir.TypeRef{Kind: types.Enum, Named: tone}}
	holder := &ir.Record{Pkg: "q", Name: "Holder", Fields: []*ir.Field{field}}
	for _, c := range []struct {
		name     string
		emits    []*ir.Emit
		reserved bool
	}{
		{"owner with a ts emit", []*ir.Emit{{Target: ir.TargetTS, Mode: ir.ModeBaked}}, true},
		{"owner with a types ts emit", []*ir.Emit{{Target: ir.TargetTS, Mode: ir.ModeTypes}}, true},
		{"owner with no ts emit", []*ir.Emit{{Target: ir.TargetGo, Mode: ir.ModeBaked}}, false},
	} {
		q := &ir.Package{Name: "q", Types: []ir.Type{holder}, Imports: []*ir.PackageRef{{Name: "base.core", Emits: c.emits}}}
		e := &ir.Emit{Target: ir.TargetTS, Mode: ir.ModeBaked}
		if got := ir.TSModuleNames(q, e)[want]; got != c.reserved {
			t.Errorf("%s: %s reserved %v, want %v", c.name, want, got, c.reserved)
		}
	}
}
