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

// hasE1702 reports a bag holding the silenced code (I18N.md F4).
func hasE1702(bag *diag.Bag) bool {
	code := diag.E1702.Def().Code
	for _, f := range bag.Findings() {
		if f.Code == code {
			return true
		}
	}
	return false
}

// I18N.md F4: a key into a broken declaration stays silent; its own finding stands alone.
func TestBrokenDeclarationKeySilencedNotCascaded(t *testing.T) {
	c := checkFiles(t, map[string]string{
		"j4a/j4a.canon": `package j4a

/// A broken record: names an unknown type.
record Bad {
  /// This does not exist.
  x: NoSuchType
}

emit view { out: "out/j4a.view.json" }
`,
		"j4a/j4a.fr.canon": `package j4a
translation fr

Bad.x "quelque chose"
`,
	})
	bag := c.bags["j4a"]
	if bag == nil {
		t.Fatal("no bag for j4a")
	}
	if hasE1702(bag) {
		t.Error("a broken declaration's key must stay silent (F4)")
	}
	if bag.Summary().Errors == 0 {
		t.Error("Bad's own finding (its unknown field type) should still be reported")
	}
}

// I18N.md F4: beside a broken sibling, a wrong key into an intact declaration is still E1702.
func TestWrongKeyIntoIntactSiblingStillE1702(t *testing.T) {
	c := checkFiles(t, map[string]string{
		"j4b/j4b.canon": `package j4b

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

emit view { out: "out/j4b.view.json" }
`,
		"j4b/j4b.fr.canon": `package j4b
translation fr

Bad.x "quelque chose"
Good.bogus "autre chose"
`,
	})
	bag := c.bags["j4b"]
	if bag == nil {
		t.Fatal("no bag for j4b")
	}
	if !hasE1702(bag) {
		t.Error("Good.bogus names no field of the intact Good: the wrong-key finding must still be reported (F4)")
	}
}

// I18N.md F4: a key naming nothing while a source file did not parse stays silent too.
func TestUnparsedFileKeySilencedNotCascaded(t *testing.T) {
	c := checkFiles(t, map[string]string{
		"j4c/j4c.canon": `package j4c

record Good {
  name: String
}

record Item {
  n: Int
`,
		"j4c/j4c.fr.canon": `package j4c
translation fr

Item.n "quelque chose"
`,
	})
	bag := c.bags["j4c"]
	if bag == nil {
		t.Fatal("no bag for j4c")
	}
	if hasE1702(bag) {
		t.Error("a key naming nothing while unparsed must stay silent (F4)")
	}
	if bag.Summary().Errors == 0 {
		t.Error("the file's own parse error should still be reported")
	}
}

// I18N.md F4: while unparsed, a wrong key into a declaration that did parse is still E1702.
func TestWrongKeyIntoParsedDeclarationWhileUnparsedStillE1702(t *testing.T) {
	c := checkFiles(t, map[string]string{
		"j4d/j4d.canon": `package j4d

/// A fine record.
record Good {
  /// Its name.
  name: String
}

record Item {
  n: Int
`,
		"j4d/j4d.fr.canon": `package j4d
translation fr

Good.bogus "autre chose"
`,
	})
	bag := c.bags["j4d"]
	if bag == nil {
		t.Fatal("no bag for j4d")
	}
	if !hasE1702(bag) {
		t.Error("Good parsed fine and has no field bogus: the wrong-key finding must still be reported even while unparsed (F4)")
	}
}

// I18N.md F4: while unparsed, a check.<n> key (a package-level check) counts as naming nothing
// too, and stays silent.
func TestUnparsedFileCheckKeySilencedNotCascaded(t *testing.T) {
	c := checkFiles(t, map[string]string{
		"j4e/j4e.canon": `package j4e

record Good {
  name: String
}

record Item {
  n: Int
`,
		"j4e/j4e.fr.canon": `package j4e
translation fr

check.lost "quelque chose"
`,
	})
	bag := c.bags["j4e"]
	if bag == nil {
		t.Fatal("no bag for j4e")
	}
	if hasE1702(bag) {
		t.Error("check.lost must stay silent while unparsed (F4)")
	}
	if bag.Summary().Errors == 0 {
		t.Error("the file's own parse error should still be reported")
	}
}

// I18N.md F4, API.md F7: "does not parse" reads the AST, never the truncated bag.Findings();
// a key naming nothing stays silent, and Summary's counts stay the same, with or without a
// limit tight enough to drop the very syntax finding this decision would otherwise read.
func TestUnparsedIgnoresTruncation(t *testing.T) {
	var summaries []diag.Summary
	for _, limit := range []int{0, 1} {
		bag, silenced := probeTruncated(t, limit)
		if silenced {
			t.Errorf("limit %d: Nope.x/Nope.y must stay silent while the package does not parse (F4)", limit)
		}
		summaries = append(summaries, bag.Summary())
	}
	if summaries[0].Errors != summaries[1].Errors || summaries[0].Warnings != summaries[1].Warnings {
		t.Errorf("Summary must count the same with and without a limit (F7): %+v vs %+v", summaries[0], summaries[1])
	}
}

// probeTruncated checks a package with both a syntax error and a check error, its bag's limit
// set to limit (unset when 0), and reports whether Nope.x or Nope.y were still reported.
func probeTruncated(t *testing.T, limit int) (*diag.Bag, bool) {
	t.Helper()
	fs := &source.FileSet{}
	bags := check.Bags{}
	src, err := fs.Add("p/p.canon", "/p/p.canon", []byte("package p\n\nconst A = 1 + \"x\"\n\n@@@ garbage\n"))
	if err != nil {
		t.Fatal(err)
	}
	trans, err := fs.Add("p/p.fr.canon", "/p/p.fr.canon", []byte("package p\ntranslation fr\n\nNope.x \"un\"\nNope.y \"deux\"\n"))
	if err != nil {
		t.Fatal(err)
	}
	bag := packageBag(bags, fs, "p/p.canon")
	files := []*syntax.File{
		syntax.Parse(src, syntax.FileSource, bag),
		syntax.Parse(trans, syntax.FileTranslation, bag),
	}
	if limit > 0 {
		bag.Truncate(limit)
	}
	proj := fixtureProject()
	prog := check.Check(context.Background(), proj, files, bags, eval.NewFolder(bags, eval.Options{}))
	i18n.Check(prog, proj, bags, emitsView(prog))
	return bag, hasE1702(bag)
}
