package ir_test

import (
	"maps"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/value"
)

// livemembersSrc is one enum and a baked table, planned by every target.
const livemembersSrc = `package a

/// A tone.
enum Tone { red, blue }

/// A badge.
record Badge {
  /// Label.
  label: String
}

/// The badges.
let badges: table Badge = {
  gold { label: "Gold" }
}

emit go { out: "@features/a", package: "a", mode: baked }
emit cpp { out: "@features/a", namespace: "a", mode: baked }
emit ts { out: "@features/a/a.ts", mode: baked }
`

// TestRetiredNamesArePlanned is CODEGEN.md §5.2, §5.3 and DECISIONS 395: each target's name plan declares the live members list and the retired lookup, and the accessors name them.
func TestRetiredNamesArePlanned(t *testing.T) {
	w := newWorld(t)
	w.add(t, "a/a.canon", []byte(livemembersSrc))
	p := w.build(t)[0]
	checks := map[ir.Target]func(*testing.T, *ir.Package, *ir.Emit){ir.TargetGo: checkGoRetired, ir.TargetCpp: checkCppRetired, ir.TargetTS: checkTSRetired}
	for _, e := range p.Emits {
		checks[e.Target](t, p, e)
	}
}

// checkGoRetired checks <E>LiveMembers and the Retired methods of the Go plan.
func checkGoRetired(t *testing.T, p *ir.Package, e *ir.Emit) {
	t.Helper()
	pl := ir.PlanGoNames(p, e)
	names := ir.GoScopeNames(pl)
	if n := pl.LiveMembersName("Tone"); n != "ToneLiveMembers" || !names["package"][n] {
		t.Errorf("go: %q planned %v", n, names["package"][n])
	}
	if !names["Tone"][ir.GoRetired] || !names["BadgeID"][ir.GoRetired] {
		t.Errorf("go: Retired not planned on Tone and the id type: %v", names)
	}
}

// checkCppRetired checks k<E>LiveMembers in the C++ plan.
func checkCppRetired(t *testing.T, p *ir.Package, e *ir.Emit) {
	t.Helper()
	names := ir.CppScopeNames(ir.PlanCppNames(p, e))
	n := ir.PlanCppNames(p, e).EnumHelpers("Tone").LiveMembers
	if n != "kToneLiveMembers" || !slices.ContainsFunc(slices.Collect(maps.Values(names)), func(s map[string]bool) bool { return s[n] }) {
		t.Errorf("cpp: %q not planned", n)
	}
}

// checkTSRetired checks <E>LiveMembers and <Id>LiveMembers in the TS plan.
func checkTSRetired(t *testing.T, p *ir.Package, e *ir.Emit) {
	t.Helper()
	names := ir.TSModuleNames(p, e)
	for _, n := range []string{ir.TSLiveMembersName("Tone"), ir.TSLiveMembersName("BadgeId")} {
		if !names[n] {
			t.Errorf("ts: %q not planned: %v", n, names)
		}
	}
}

// TestEntryRetired is CODEGEN.md §5.3 and DECISIONS 395: EntryRetired reads the entry's identity, and is false for a non-table value and an index out of range.
func TestEntryRetired(t *testing.T) {
	tbl := &value.Table{Entries: []*value.Record{{Ident: &value.Identity{}}, {Ident: &value.Identity{Retired: true}}}}
	tests := []struct {
		name string
		v    *ir.Value
		i    int
		want bool
	}{
		{"live", &ir.Value{V: tbl}, 0, false},
		{"retired", &ir.Value{V: tbl}, 1, true},
		{"past the end", &ir.Value{V: tbl}, 2, false},
		{"negative", &ir.Value{V: tbl}, -1, false},
		{"not a table", &ir.Value{V: &value.Int{}}, 0, false},
	}
	for _, tc := range tests {
		if got := ir.EntryRetired(tc.v, tc.i); got != tc.want {
			t.Errorf("%s: EntryRetired = %v, want %v", tc.name, got, tc.want)
		}
	}
}
