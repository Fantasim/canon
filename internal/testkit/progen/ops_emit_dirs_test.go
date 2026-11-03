package progen_test

import (
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/testkit/progen"
)

// sharedGoDir points a package's go emit at the directory of an imported package's.
func sharedGoDir(tg target) []progen.Site {
	var out []progen.Site
	for _, other := range importedGoEmits(tg) {
		for _, s := range emitValueOf(tg, "go", "out", `"`+other.dir+`"`) {
			s.Packages = []string{tg.pkg, other.pkg}
			out = append(out, s)
		}
	}
	return out
}

// sharedGoDirOp is the E8008 operator, whose sites point one Go emit at another's directory.
func sharedGoDirOp() operator {
	return op(diag.E8008.Def().Code, "CODEGEN.md §2.3 (two Go emits, one directory)", sharedGoDir)
}

// goDir is a package and the directory its go emit writes, named from the file to mutate.
type goDir struct{ pkg, dir string }

// importedGoEmits are the go emit directories of the packages tg imports that sort before its own.
func importedGoEmits(tg target) []goDir {
	var out []goDir
	for _, imp := range nodes[*syntax.Import](tg) {
		for _, other := range *tg.all {
			if other.pkg == text(tg, imp.Path) && other.pkg < tg.pkg {
				out = append(out, goDirs(tg, other)...)
			}
		}
	}
	return out
}

// listOut reports whether d's `out` is a list literal, which names no one directory (CODEGEN.md §2.1).
func listOut(d *syntax.EmitDecl) bool {
	f := option(d, "out")
	if f == nil {
		return false
	}
	_, isList := f.Value.(*syntax.ListLit)
	return isList
}

// goDirs are the directories other's go emits write, named from tg (CODEGEN.md §2.1: unrooted, from its file).
func goDirs(tg, other target) []goDir {
	var out []goDir
	for _, d := range emitsOf(other, "go") {
		if listOut(d) {
			continue
		}
		if dir := optionText(other, d, "out"); dir != "" {
			out = append(out, goDir{other.pkg, strings.TrimSuffix(fromDir(tg, other, dir), "/")})
		}
	}
	return out
}

// TestSharedGoDirEverySite proves each E8008 operator site, not one seeded pick (CODEGEN.md §2.3).
func TestSharedGoDirEverySite(t *testing.T) {
	c := examples(t)
	o := sharedGoDirOp()
	for _, p := range o.placements(c) {
		m, err := p.site.Mutate(c.project)
		if err != nil {
			t.Fatalf("%s: %v", p.site.Path, err)
		}
		if v := judge(o, p.run, m.At, m.Project, c.baselineOf(p.run.pkgs)); v.Kind != "" {
			t.Errorf("%s %v %q: %s", p.site.Path, p.run.pkgs, p.site.Edits[0].Text, v.Text)
		}
	}
}

// CODEGEN.md §2.1: `out` may be a list, which names no one directory; a string or no `out` is not a list.
func TestListOut(t *testing.T) {
	for _, tc := range []struct {
		name, emit string
		want       bool
	}{
		{"list", `emit go { out: ["a/", "b/"] }`, true},
		{"string", `emit go { out: "a/" }`, false},
		{"none", `emit go { mode: baked }`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tg := target{file: parse("p/p.canon", []byte("package p\n\n"+tc.emit+"\n"))}
			emits := emitsOf(tg, "go")
			if len(emits) != 1 {
				t.Fatalf("%d go emits parsed, want one", len(emits))
			}
			if got := listOut(emits[0]); got != tc.want {
				t.Errorf("listOut = %v, want %v", got, tc.want)
			}
		})
	}
}
