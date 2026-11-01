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

// goDirs are the directories other's go emits write, named from tg (CODEGEN.md §2.1: unrooted, from its file).
func goDirs(tg, other target) []goDir {
	var out []goDir
	for _, d := range emitsOf(other, "go") {
		if dir := optionText(other, d, "out"); dir != "" {
			out = append(out, goDir{other.pkg, strings.TrimSuffix(fromDir(tg, other, dir), "/")})
		}
	}
	return out
}

// TestSharedGoDirEverySite proves each E8008 operator site, not one seeded pick (CODEGEN.md §2.3).
func TestSharedGoDirEverySite(t *testing.T) {
	c := examples(t)
	o := op(diag.E8008.Def().Code, "CODEGEN.md §2.3 (two Go emits, one directory)", sharedGoDir)
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
