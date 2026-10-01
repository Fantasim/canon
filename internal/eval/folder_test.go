package eval_test

import (
	"context"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
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

// foldedLib is package b: N, which a's bound folds, reads L, an orphan ref.
const foldedLib = "/// B.\npackage b\n\n/// Node.\nrecord Node {\n  /// Name.\n  name: String\n}\n\n" +
	"/// Link.\nrecord Link {\n  /// To.\n  to: ref Node\n}\n\n/// Tree.\nrecord Tree {\n  /// Nodes.\n" +
	"  nodes: [Node] keyed by name\n  /// Links.\n  links: [Link]\n}\n\n/// An orphan link.\n" +
	"const L = Link { to: \"x\" }\n\n/// How many.\nconst N = [L, L].len()\n"

// foldedUser is package a: only P's bound reads b.
const foldedUser = "/// A.\npackage a\n\nimport b\n\n/// A thing.\nrecord P {\n  /// Its x.\n" +
	"  x: Int(0..=b.N)\n}\n\n/// Things.\nlet ps: [P] = [{ x: 1 }]\n"

// DECISIONS 264, EVALUATION.md §2.1, §5: with a alone selected, stage A forces b.N and b.L, which a fold read.
func TestFoldReadsForcedUnselected(t *testing.T) {
	files := parseFiles(t, []string{"a/a.canon", "b/b.canon"}, [][]byte{[]byte(foldedUser), []byte(foldedLib)})
	b := runBuild(t, files, eval.Options{}, "a")
	for _, name := range []string{"N", "L"} {
		if _, ok := b.ev.Settled(eval.Root{Pkg: "b", Name: name}); !ok {
			t.Errorf("b.%s is not forced", name)
		}
	}
	if f := b.bags["b"].Findings(); len(f) != 1 || f[0].Code != diag.E3505.Def().Code {
		t.Errorf("b's findings %v, want the orphan ref's", f)
	}
}

// readsSource declares constants whose values name others, to fold.
const readsSource = "/// A.\npackage a\n\n/// A.\nconst A = 1\n\n/// B.\nconst B = A + 1\n\n/// C.\nconst C = 3\n\n" +
	"/// To B.\nconst RB = B\n\n/// To C.\nconst RC = C\n\n/// To A.\nconst RA = A\n"

// EVALUATION.md §2.1, DECISIONS 264: Reads is each constant the folds read, once, in fold order.
func TestFoldsReadsInFoldOrder(t *testing.T) {
	// Constants read through others count; a snapshot holds only the folds made before it.
	b := runBuild(t, parseFiles(t, []string{"a/a.canon"}, [][]byte{[]byte(readsSource)}), eval.Options{})
	decls := map[string]check.Object{}
	for _, obj := range b.checked.Packages[0].Decls {
		decls[obj.Name()] = obj
	}
	fold := eval.NewFolder(b.bags, eval.Options{})
	foldOf := func(name string) {
		obj := decls[name]
		if _, ok := fold.Fold(context.Background(), obj, obj.Decl().(*syntax.ConstDecl).Value, b.checked.Info); !ok {
			t.Fatalf("%s did not fold", name)
		}
	}
	foldOf("RC")
	first := eval.FoldsOf(fold)
	foldOf("RB") // reads B, and A through it
	second := eval.FoldsOf(fold)
	for _, name := range []string{"RA", "RC"} { // read already: nothing new
		foldOf(name)
	}
	want := []eval.Root{{Pkg: "a", Name: "C"}, {Pkg: "a", Name: "B"}, {Pkg: "a", Name: "A"}}
	if got := first.Reads(); !slices.Equal(got, want[:1]) {
		t.Errorf("the first snapshot reads %v, want %v", got, want[:1])
	}
	if got := second.Reads(); !slices.Equal(got, want) {
		t.Errorf("the snapshot after RB reads %v, want %v", got, want)
	}
	if got := eval.FoldsOf(fold).Reads(); !slices.Equal(got, want) {
		t.Errorf("reads %v, want %v", got, want)
	}
	if got := eval.FoldsOf(otherFolder{}).Reads(); len(got) != 0 {
		t.Errorf("another folder reads %v", got)
	}
}
