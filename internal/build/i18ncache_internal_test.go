package build

import (
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
)

const (
	badRoot     = "/law"
	badEdited   = "/law/a/a.canon"
	badGood     = "package a\n\n/// Good.\nrecord Good {\n  /// Name.\n  name: String\n}\n"
	badBroken   = badGood + "\nrecord Item {\n  n: Int\n"
	badProject  = "project acme {\n  canon: \"0.1\"\n  languages: [en, fr]\n}\n"
	badOther    = "package a\n\n/// Other.\nrecord Other {\n  /// Name.\n  name: String\n}\n"
	badTranslFr = "package a\ntranslation fr\n\nItem.n \"quelque chose\"\n"
)

// badCaseSteps are the edited file's contents in order; wantE1702 is whether the unknown key in
// the French file is reported (I18N.md F4: silent while a source file did not parse).
var badCaseSteps = []struct {
	name      string
	src       string
	wantE1702 bool
}{
	{"parses", badGood, true},
	{"bad node introduced", badBroken, false},
	{"bad node kept", badBroken + "  m: Int\n", false},
	{"bad node removed", badGood, true},
	{"parses again", badGood + "\n/// Extra.\nlet x: Int = 1\n", true},
}

// I18N.md F4, IMPLEMENTATION-PLAN §7.6: an edit adding or removing a Bad node re-checks as cold.
func TestI18NBadNodeCacheEqualsCold(t *testing.T) {
	base := roFS{
		"law/project.canon": srcFile(badProject),
		"law/a/a.canon":     srcFile(badGood),
		"law/a/other.canon": srcFile(badOther),
		"law/a/a.fr.canon":  srcFile(badTranslFr),
	}
	z := &analyzer{fs: newEditFS(base), dir: badRoot, cache: NewCache()}
	e1702 := diag.E1702.Def().Code
	for _, st := range badCaseSteps {
		z.fs.set(badEdited, []byte(st.src))
		warm, cold := z.pair(t)
		same(t, st.name, warm, cold)
		got := false
		for _, f := range warm.Result().List {
			got = got || f.Code == e1702
		}
		if got != st.wantE1702 {
			t.Errorf("%s: unknown-key finding reported %t, want %t", st.name, got, st.wantE1702)
		}
	}
}
