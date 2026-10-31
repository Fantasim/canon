package cppgen

import (
	"maps"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
)

// guardNames are G_A, G_B, IMP_A, IMP_B: two own, two read, one of them shared.
const guardNames = 4

// guardPackage is a package whose enum writes G_A and G_B as defines and that reads two packages, one writing G_B again and IMP_A, the other IMP_B.
func guardPackage() *ir.Package {
	own := &ir.Enum{
		Pkg: "guard.own", Name: "Own", CppDefines: "G_", HasCppDefines: true,
		Members: []*ir.EnumMember{{Name: "B", Wire: "B", Index: 0}, {Name: "A", Wire: "A", Index: 1}},
	}
	return &ir.Package{
		Name: "guard.own", Dir: "guard/own", Types: []ir.Type{own},
		Imports: []*ir.PackageRef{
			{Name: "guard.one", CppDefines: []string{"G_B", "IMP_A"}},
			{Name: "guard.two", CppDefines: []string{"IMP_B"}},
		},
	}
}

// CODEGEN.md §7.9, DECISIONS 391, 403: the macros gen/cpp wraps are exactly ir.CppGuardNames, the set the foreign-define rule excepts, on a package with imports.
func TestMacroSetIsCppGuardNames(t *testing.T) {
	t.Parallel()
	p := guardPackage()
	g := &gen{p: p}
	got := slices.Sorted(maps.Keys(g.macroSet()))
	want := ir.CppGuardNames(p)
	if !slices.Equal(got, want) {
		t.Errorf("macroSet %v, ir.CppGuardNames %v", got, want)
	}
	if len(want) != guardNames {
		t.Errorf("%d guard names %v, want %d", len(want), want, guardNames)
	}
}
