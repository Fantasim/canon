package build_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
	"golang.org/x/tools/txtar"
)

// EVALUATION.md §2.3, DECISIONS 324: check and build report a precomputed result's findings alike.
func TestPrecomputedCheckBuild(t *testing.T) {
	files, err := filepath.Glob("testdata/findings/*_precomputed*.txtar")
	if err != nil || len(files) == 0 {
		t.Fatalf("no precomputed cases: %v", err)
	}
	ctx := context.Background()
	for _, f := range files {
		a, err := txtar.ParseFile(f)
		if err != nil {
			t.Fatal(err)
		}
		checked, err := open(t, a).Check(ctx, nil)
		if err != nil {
			t.Fatal(err)
		}
		built, err := open(t, a).Build(ctx, build.BuildOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if c, b := render(t, checked.Findings), render(t, built.Findings); c != b {
			t.Errorf("%s: check reports\n%s\nbuild reports\n%s", f, c, b)
		}
	}
}

const (
	langResultA = `/// A.
package a

/// Pick.
record Pick {
  /// N.
  n: Int

  check positive: n > 0 else "n {n} is not positive"
}

/// Hold.
record Hold {
  /// N.
  n: Int = 1

  /// A pick failing its named check.
  export fn bad(self) -> Pick { return { n: n - 1 } }
}

/// Holds.
let holds: table Hold = { h {} }

emit json { out: "out/" }
`
	langResultFr    = "package a\ntranslation fr\n\nPick.check.positive \"n {n} n'est pas positif\"\n"
	langResultFrMsg = "n 0 n'est pas positif"
)

// I18N.md B5, DECISIONS 324: a named check failing on a precomputed result is translated under --lang.
func TestLangPrecomputed(t *testing.T) {
	fsys := mapFS{"p/project.canon": file(langProject), "p/a/a.canon": file(langResultA), "p/a/a.fr.canon": file(langResultFr)}
	p, err := build.Open(fsys, "/p", build.Options{Lang: "fr"})
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Check(context.Background(), []string{"a"})
	if err != nil {
		t.Fatal(err)
	}
	if got := messageOf(t, res.List, "positive", diag.E5001.Def().Code); got != langResultFrMsg {
		t.Errorf("message %q, want %q", got, langResultFrMsg)
	}
}

func open(t *testing.T, a *txtar.Archive) *build.Project {
	t.Helper()
	p, err := build.Open(archiveFS(a), "/p", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	return p
}
