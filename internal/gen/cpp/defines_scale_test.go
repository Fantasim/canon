package cppgen_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

const (
	scaleMembers = 20000
	scalePrefix  = "BD_"
	scaleBound   = 120 * time.Second
	genHeader    = "defs.gen.h"
	userHeader   = "user.gen.h"
	userSource   = "user.gen.cpp"
	scaleMacro   = "#pragma push_macro(\"BD_M"
)

// scalePackages are big.defs, an enum of scaleMembers members with @cpp(defines:) and big.user, a types-mode importer whose Holder defaults a field to the first member (the importer spells one foreign enumerator).
func scalePackages() (owner, user *ir.Package) {
	members := make([]*ir.EnumMember, scaleMembers)
	for i := range members {
		name := fmt.Sprintf("BD_M%d", i)
		members[i] = &ir.EnumMember{Name: name, Wire: name, Index: i}
	}
	enum := &ir.Enum{Pkg: "big.defs", Name: "Big", CppDefines: scalePrefix, HasCppDefines: true, Members: members}
	emit := &ir.Emit{Target: ir.TargetCpp, Out: "out/", Dir: "big/defs/out", Mode: ir.ModeBaked, Namespace: "big::defs"}
	owner = &ir.Package{Name: "big.defs", Dir: "big/defs", Types: []ir.Type{enum}, Emits: []*ir.Emit{emit}}
	big := field("big", "big", "", ir.TypeRef{Kind: types.Enum, Named: enum})
	big.Default = &value.Member{Index: 0}
	holder := &ir.Record{Pkg: "big.user", Name: "Holder", Fields: []*ir.Field{big}}
	userEmit := &ir.Emit{Target: ir.TargetCpp, Out: "out/", Dir: "big/user/out", Mode: ir.ModeTypes, Namespace: "big::user"}
	user = &ir.Package{
		Name: "big.user", Dir: "big/user", Types: []ir.Type{holder}, Emits: []*ir.Emit{userEmit},
		Imports: []*ir.PackageRef{{Name: owner.Name, Dir: owner.Dir, Emits: owner.Emits, CppDefines: ir.CppDefineNames(owner)}},
	}
	return owner, user
}

// CODEGEN.md §7.9: wrapping every file against the macros it names is linear in the text; one enum of 20,000 defines, generated for its owner and an importer, takes ~2 s (~30 s under -race and load); the bound is loose, ~60x that time, and only the quadratic form (4 minutes) exceeds it.
func TestDefinesClashDetectionScales(t *testing.T) {
	if testing.Short() {
		t.Skip("generates an enum of 20,000 members")
	}
	owner, user := scalePackages()
	start := time.Now()
	files := generate(t, owner)
	files = append(files, generate(t, user)...)
	elapsed := time.Since(start)
	t.Logf("%d members: owner and importer generated twice each in %v", scaleMembers, elapsed)
	if elapsed > scaleBound {
		t.Errorf("generation took %v, over %v", elapsed, scaleBound)
	}
	wrapped, header, foreign := 0, "", ""
	for _, f := range files {
		switch {
		case strings.HasSuffix(f.Path, genHeader):
			wrapped = strings.Count(string(f.Content), scaleMacro)
		case strings.HasSuffix(f.Path, userHeader):
			header = string(f.Content)
		case strings.HasSuffix(f.Path, userSource):
			foreign = string(f.Content)
		}
	}
	if wrapped != scaleMembers {
		t.Errorf("the owner's header wraps %d of %d macros", wrapped, scaleMembers)
	}
	if header == "" {
		t.Fatalf("no %s was generated", userHeader)
	}
	if got := pushedMacros(header); len(got) > 0 {
		t.Errorf("the importer's header wraps %v, want none: the default naming the foreign enumerator is built in the source", got)
	}
	if got := pushedMacros(foreign); !slices.Equal(got, []string{scalePrefix + "M0"}) {
		t.Errorf("the importer's source wraps %v, want only the foreign enumerator it spells", got)
	}
}
