package i18n_test

import (
	"context"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/i18n"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// I18N.md W1: a package with an emit view but not selected gets no W1701 (emitsView is the
// selected packages that have one; build computes it, U8).
func TestUnselectedPackageNoW1701(t *testing.T) {
	fs := &source.FileSet{}
	parse := diag.NewBag(fs, "")
	src, err := fs.Add("sel/sel.canon", "/sel/sel.canon", []byte(`package sel

/// A thing.
record Thing {
  /// Its name.
  name: String
}

emit view { out: "out/sel.view.json" }
`))
	if err != nil {
		t.Fatal(err)
	}
	file := syntax.Parse(src, syntax.FileSource, parse)
	bags := check.Bags{}
	proj := fixtureProject()
	prog := check.Check(context.Background(), proj, []*syntax.File{file}, bags, eval.NewFolder(bags, eval.Options{}))
	res := i18n.Check(prog, proj, bags, map[string]bool{}) // sel not selected
	if res["sel"] == nil || res["sel"].Languages["fr"].Missing == 0 {
		t.Fatal("sel should still have a catalogue and a missing count")
	}
	total := 0
	for _, b := range bags { //canon:unordered a sum
		total += len(b.Findings())
	}
	if total != 0 {
		t.Errorf("an unselected package should report nothing, got %d findings", total)
	}
}

// I18N.md W1: a package without `emit view` gets no W1701, but its translation files are still
// checked and its catalogue still computed (its keys still counted by `canon i18n status`).
func TestNoEmitViewNoW1701(t *testing.T) {
	c := checkFiles(t, map[string]string{"nv/nv.canon": `package nv

/// A thing.
record Thing {
  /// Its name.
  name: String
}
`})
	res := c.res["nv"]
	if res == nil {
		t.Fatal("no result for nv")
	}
	if got := res.Languages["fr"].Missing; got != len(res.Catalogue.Entries) {
		t.Errorf("Missing = %d, want %d (still counted, W10)", got, len(res.Catalogue.Entries))
	}
	if out := c.render(t); out != "0 errors, 0 warnings in 1 package (…)\n" {
		t.Errorf("no emit view should report nothing:\n%s", out)
	}
}

// VIEWMODEL.md J4: a broken declaration is left out of the catalogue and of W1701's count.
func TestBrokenDeclarationLeftOutOfCatalogue(t *testing.T) {
	c := checkFiles(t, map[string]string{"j4/j4.canon": `package j4

/// A broken record: names an unknown type.
record Bad {
  /// This does not exist.
  x: NoSuchType
}

/// A fine record.
record Good {
  /// Its name.
  name: String
}

emit view { out: "out/j4.view.json" }
`})
	res := c.res["j4"]
	if res == nil {
		t.Fatal("no result for j4")
	}
	for _, key := range []string{"Bad.help", "Bad.x", "Bad.x.help"} {
		if _, ok := res.Catalogue.Lookup(key); ok {
			t.Errorf("a broken record's key %s should be left out of the catalogue (J4)", key)
		}
	}
	for _, key := range []string{"Good.help", "Good.name", "Good.name.help"} {
		if _, ok := res.Catalogue.Lookup(key); !ok {
			t.Errorf("missing %s in %v", key, keys(res.Catalogue))
		}
	}
	if got, want := len(res.Catalogue.Entries), 3; got != want {
		t.Errorf("catalogue has %d keys, want %d (Bad's are left out)", got, want)
	}
	if got := res.Languages["fr"].Missing; got != 3 {
		t.Errorf("Missing = %d, want 3 (the missing count excludes the broken record too)", got)
	}
}

// I18N.md F5: an empty translation ("") counts as missing.
func TestEmptyTranslationCountsAsMissing(t *testing.T) {
	src := map[string]string{
		"em/em.canon": `package em

/// A thing.
record Thing {
  /// Its name.
  name: String
}

emit view { out: "out/thing.view.json" }
`,
		"em/em.fr.canon": `package em
translation fr

Thing.name ""
`,
	}
	res := checkFiles(t, src).res["em"]
	if _, ok := res.Languages["fr"].Texts["Thing.name"]; ok {
		t.Errorf("an empty translation should not be recorded as translated")
	}
	if got, want := res.Languages["fr"].Missing, len(res.Catalogue.Entries); got != want {
		t.Errorf("Missing = %d, want %d (the empty entry still counts as missing)", got, want)
	}
}
