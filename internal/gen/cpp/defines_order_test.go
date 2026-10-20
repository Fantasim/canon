package cppgen_test

import (
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

const orderPrefix = "DP_"

// pushPattern finds the macros a file saves with #pragma push_macro, in file order.
var pushPattern = regexp.MustCompile(`#pragma push_macro\("([^"]+)"\)`)

// pushedMacros are the macros text wraps, in the order it wraps them.
func pushedMacros(text string) []string {
	var out []string
	for _, m := range pushPattern.FindAllStringSubmatch(text, -1) {
		out = append(out, m[1])
	}
	return out
}

// defineEnum is an enum of pkg with @cpp(defines: "DP_") whose members are all spelled with the prefix.
func defineEnum(pkg, name string, members ...string) *ir.Enum {
	e := &ir.Enum{Pkg: pkg, Name: name, CppDefines: orderPrefix, HasCppDefines: true}
	for i, m := range members {
		e.Members = append(e.Members, &ir.EnumMember{Name: m, Wire: m, Index: i})
	}
	return e
}

// orderPackages are dup.defs, a defines enum Far, and dup.user, a types-mode package with its own defines enum Near whose fields default to every member of both; the enums share DP_X, first in both.
func orderPackages() (owner, user *ir.Package) {
	far := defineEnum("dup.defs", "Far", "DP_X", "DP_A")
	emit := &ir.Emit{Target: ir.TargetCpp, Out: "out/", Dir: "dup/defs/out", Mode: ir.ModeBaked, Namespace: "dup::defs"}
	owner = &ir.Package{Name: "dup.defs", Dir: "dup/defs", Types: []ir.Type{far}, Emits: []*ir.Emit{emit}}
	near := defineEnum("dup.user", "Near", "DP_X", "DP_B")
	var fields []*ir.Field
	for _, e := range []*ir.Enum{far, near} {
		for i, m := range e.Members {
			f := field(strings.ToLower(e.Name+m.Name), strings.ToLower(e.Name+m.Name), "", ir.TypeRef{Kind: types.Enum, Named: e})
			f.Default = &value.Member{Index: i}
			fields = append(fields, f)
		}
	}
	holder := &ir.Record{Pkg: "dup.user", Name: "Holder", Fields: fields}
	userEmit := &ir.Emit{Target: ir.TargetCpp, Out: "out/", Dir: "dup/user/out", Mode: ir.ModeTypes, Namespace: "dup::user"}
	user = &ir.Package{
		Name: "dup.user", Dir: "dup/user", Types: []ir.Type{near, holder}, Emits: []*ir.Emit{userEmit},
		Imports: []*ir.PackageRef{{Name: owner.Name, Dir: owner.Dir, Emits: owner.Emits, CppDefines: ir.CppDefineNames(owner)}},
	}
	return owner, user
}

// CODEGEN.md §7.9, DECISIONS 391: a file naming the enumerators of an own and an imported defines enum wraps each macro once, in sorted order, own and imported alike; the header names only the own enum's, the source (the record defaults) both.
func TestDefinesWrapOrderSorted(t *testing.T) {
	t.Parallel()
	_, user := orderPackages()
	wrapped := map[string][]string{}
	for _, f := range generate(t, user) {
		wrapped[f.Path] = pushedMacros(string(f.Content))
		if got, pops := wrapped[f.Path], strings.Count(string(f.Content), "#pragma pop_macro"); pops != len(got) {
			t.Errorf("%s: %d push_macro lines, %d pop_macro lines", f.Path, len(got), pops)
		}
	}
	for _, c := range []struct {
		path string
		want []string
	}{{userHeader, []string{"DP_B", "DP_X"}}, {userSource, []string{"DP_A", "DP_B", "DP_X"}}} {
		if !slices.Equal(wrapped[c.path], c.want) {
			t.Errorf("%s wraps %v, want %v", c.path, wrapped[c.path], c.want)
		}
	}
}

// afterMacro is the one identifier of an opened enum's std:: specialisation that no other region of the header spells.
const afterMacro = "hash"

// CODEGEN.md §7.9, DECISIONS 391: the region after the header's namespace (the std::hash of an opened enum) is wrapped too: a macro that only that region names is saved before the namespace and restored after the region.
func TestDefinesWrapCoversAfterNamespace(t *testing.T) {
	t.Parallel()
	p := openPackage()
	// the define is the prefix `ha` and the member `sh`, so the text spells neither the macro nor a name that holds it
	p.Types = append(p.Types, &ir.Enum{Pkg: openPkg, Name: "Spare", CppDefines: "ha", HasCppDefines: true, Members: members([]string{"sh", "sh"}, nil)})
	var header string
	for _, f := range generate(t, p) {
		if f.Path == "open.gen.h" {
			header = string(f.Content)
		}
	}
	push, spec, pop := strings.Index(header, `#pragma push_macro("`+afterMacro+`")`), strings.Index(header, "struct std::"+afterMacro), strings.Index(header, `#pragma pop_macro("`+afterMacro+`")`)
	if push < 0 || spec < 0 || pop < 0 || push > spec || spec > pop {
		t.Errorf("push at %d, specialisation at %d, pop at %d, want them in that order", push, spec, pop)
	}
}

// CODEGEN.md §7.9, DECISIONS 391: a define equal to a segment of the emit namespace is wrapped in every file, though no other token of the .gen.cpp spells it (the enumerator is NS, the macro LIMIT_NS).
func TestDefinesWrapCoversNamespaceSegment(t *testing.T) {
	t.Parallel()
	p := definesPackage()
	p.Emits[0].Namespace = "demo::LIMIT_NS"
	p.Types = append(p.Types, definesEnum("Level", "LIMIT_", 12, nil, members([]string{"NS", "NS"}, nil)))
	for _, f := range generate(t, p) {
		if f.Path != "defs.gen.h" && f.Path != "defs.gen.cpp" {
			continue
		}
		text := string(f.Content)
		save, open := strings.Index(text, `#pragma `+"push_macro"+`("LIMIT_NS")`), strings.Index(text, "namespace demo::LIMIT_NS {")
		if save < 0 || open < 0 || save > open {
			t.Errorf("%s: save at %d, namespace at %d, want the save first", f.Path, save, open)
		}
	}
}
