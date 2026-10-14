package ir_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
)

// n11Package is a package with an enum, a record and a public table of it.
func n11Package() *ir.Package {
	status := &ir.Record{Pkg: "p", Name: "Status"}
	tone := &ir.Enum{Pkg: "p", Name: "Tone", Members: []*ir.EnumMember{{Name: "soft", Wire: "soft"}}}
	table := ir.TypeRef{Kind: types.Table, Elem: &ir.TypeRef{Kind: types.Record, Named: status}}
	return &ir.Package{Name: "p", Types: []ir.Type{tone, status}, Values: []*ir.Value{{Name: "statuses", Type: table, IDs: []string{"open"}}}}
}

// TestGoPlanReservesIDMethods is CODEGEN.md §5.2, §5.3 and T18 (DECISIONS 379): a baked id reserves String, MarshalText, UnmarshalText and Position, an enum the first three and not Position.
func TestGoPlanReservesIDMethods(t *testing.T) {
	p := n11Package()
	codes := ir.TypeRef{Kind: types.Int, Bits: 8}
	p.Types = append(p.Types,
		&ir.Enum{Pkg: "p", Name: "Coded", Codes: &codes, Members: []*ir.EnumMember{{Name: "a", Wire: "a", Code: 1}}},
		&ir.Enum{Pkg: "p", Name: "Opened", Members: []*ir.EnumMember{{Name: "a", Wire: "a"}}})
	scopes := ir.GoScopeNames(ir.PlanGoNames(p, &ir.Emit{Target: ir.TargetGo, Mode: ir.ModeBaked}))
	for scope, names := range ir.GoScopeNames(ir.PlanGoNames(p, &ir.Emit{Target: ir.TargetGo, Mode: ir.ModeTypes, Open: []string{"Opened"}})) { //canon:unordered each scope is copied alone
		if scope == "Opened" {
			scopes[scope] = names
		}
	}
	for _, c := range []struct {
		scope string
		want  map[string]bool
	}{
		{"StatusID", map[string]bool{"String": true, "MarshalText": true, "UnmarshalText": true, "Position": true}},
		{"Tone", map[string]bool{"String": true, "Wire": true, "MarshalText": true, "UnmarshalText": true, "Position": false}},
		{"Coded", map[string]bool{"String": true, "Wire": true, "Code": true, "MarshalText": false, "UnmarshalText": false}},
		{"Opened", map[string]bool{"String": true, "Wire": true, "Known": true, "MarshalText": false, "UnmarshalText": false}},
	} {
		for name, want := range c.want { //canon:unordered each name is checked on its own
			if got := scopes[c.scope][name]; got != want {
				t.Errorf("%s reserves %s: %v, want %v", c.scope, name, got, want)
			}
		}
	}
}

// TestTSPlanReservesIDMembers is CODEGEN.md §5.3 (DECISIONS 379): a baked table reserves <Id>Members and the private <Id>Index.
func TestTSPlanReservesIDMembers(t *testing.T) {
	e := &ir.Emit{Target: ir.TargetTS, Mode: ir.ModeBaked}
	names := ir.TSModuleNames(n11Package(), e)
	for _, n := range []string{"StatusId", "StatusIdMembers", "StatusIdIndex"} {
		if !names[n] {
			t.Errorf("own table: %s is not reserved", n)
		}
	}
}

// TestTSPlanReservesForeignIndex is CODEGEN.md §2.8, §5.3 (DECISIONS 379): a ts file in any mode that holds a ref into another package's baked or embedded table reserves <alias>_<Id>Index once, however many refs reach it; an owner in data or types mode has no Members, so none is reserved.
func TestTSPlanReservesForeignIndex(t *testing.T) {
	const want = "base_core_BadgeIdIndex"
	owner := &ir.Record{Pkg: "base.core", Name: "Badge"}
	ref := &ir.RefTarget{Coll: types.CollLet, Pkg: "base.core", Value: "badges", Elem: owner}
	if got := ir.TSForeignIDIndexName(ref); got != want {
		t.Fatalf("TSForeignIDIndexName = %q, want %q", got, want)
	}
	field := func(name string) *ir.Field { return &ir.Field{Name: name, Type: ir.TypeRef{Kind: types.Ref, Ref: ref}} }
	for _, c := range []struct {
		name        string
		file, owner ir.Mode
		refs        int
		reserved    bool
	}{
		{"baked file, baked owner", ir.ModeBaked, ir.ModeBaked, 1, true},
		{"data file, baked owner", ir.ModeData, ir.ModeBaked, 1, true},
		{"types file, embedded owner", ir.ModeTypes, ir.ModeEmbedded, 1, true},
		{"data file, two refs to one owner", ir.ModeData, ir.ModeBaked, 2, true},
		{"data file, data owner", ir.ModeData, ir.ModeData, 1, false},
		{"baked file, types owner", ir.ModeBaked, ir.ModeTypes, 1, false},
	} {
		fields := []*ir.Field{field("a")}
		if c.refs > 1 {
			fields = append(fields, field("b"))
		}
		user := &ir.Record{Pkg: "q", Name: "Holder", Fields: fields}
		q := &ir.Package{Name: "q", Types: []ir.Type{user}, Imports: []*ir.PackageRef{{Name: "base.core", Emits: []*ir.Emit{{Target: ir.TargetTS, Mode: c.owner}}}}}
		e := &ir.Emit{Target: ir.TargetTS, Mode: c.file}
		if got := ir.TSModuleNames(q, e)[want]; got != c.reserved {
			t.Errorf("%s: %s reserved %v, want %v", c.name, want, got, c.reserved)
		}
		if n := ir.TSNameProblems(q, e); n != 0 {
			t.Errorf("%s: %d name problems, want none", c.name, n)
		}
	}
}
