package eval_test

import (
	"context"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/syntax"
)

// foldSource declares one const to own the folds below.
const foldSource = "/// A.\npackage a\n\n/// One.\nconst ONE = 1\n"

// DECISIONS 210: an internal error met by a fold is never silent: FoldErr hands it to the build.
func TestFoldErrSurfacesBugs(t *testing.T) {
	b := runBuild(t, parseFiles(t, []string{"a/a.canon"}, [][]byte{[]byte(foldSource)}), eval.Options{})
	owner := b.checked.Packages[0].Decls[0]
	fold := eval.NewFolder(b.bags, eval.Options{})
	if err := eval.FoldErr(fold); err != nil {
		t.Fatalf("a fresh folder has %v", err)
	}
	untyped := &syntax.IdentExpr{Name: "nowhere"} // a name the checker never resolved
	if _, ok := fold.Fold(context.Background(), owner, untyped, b.checked.Info); ok {
		t.Fatal("an unresolved name folded")
	}
	if eval.FoldErr(fold) == nil {
		t.Error("the fold's internal error was dropped")
	}
	if eval.FoldErr(otherFolder{}) != nil {
		t.Error("a folder that is not NewFolder's has no internal errors")
	}
}

// otherFolder is a check.Folder that is not NewFolder's.
type otherFolder struct{ check.Folder }
