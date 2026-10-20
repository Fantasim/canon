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
	e := &ir.Enum{Pkg: pkg, Name: name, CppDefines: orderPrefix}
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
		Imports: []*ir.PackageRef{{Name: owner.Name, Dir: owner.Dir, Emits: owner.Emits}},
	}
	return owner, user
}

// CODEGEN.md §7.9: a file naming the enumerators of an own and an imported defines enum wraps each macro once, the own enum's first, then the imported one's, members in declaration order (not sorted: Near declares DP_X before DP_B); the header names only the own enum's, the source (the record defaults) both.
func TestDefinesClashOrderOwnThenForeign(t *testing.T) {
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
	}{{userHeader, []string{"DP_X", "DP_B"}}, {userSource, []string{"DP_X", "DP_B", "DP_A"}}} {
		if !slices.Equal(wrapped[c.path], c.want) {
			t.Errorf("%s wraps %v, want %v", c.path, wrapped[c.path], c.want)
		}
	}
}
