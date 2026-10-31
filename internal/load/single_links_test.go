package load_test

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/load"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"golang.org/x/tools/txtar"
)

// skipWithoutSymlinks skips t where this machine cannot make a symbolic link (Windows without the privilege).
func skipWithoutSymlinks(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	if err := os.Symlink(filepath.Join(dir, "target"), filepath.Join(dir, "link")); err != nil {
		t.Skipf("symbolic links cannot be made here: %v", err)
	}
}

// singleLinkFiles is TestSingleFileLinkBounds' tree: files in the project, in another root, outside both.
var singleLinkFiles = map[string]string{
	"proj/plain.txt": "plain",
	"proj/sub/x.txt": "inside",
	"other/y.txt":    "other",
	"outside/z.txt":  "outside",
}

// singleLinkLinks are its links: inside the project, into the root, outside, a directory link
// outside, a dangling one and a loop.
var singleLinkLinks = map[string]string{
	"proj/in.txt":    "proj/sub/x.txt",
	"proj/root.txt":  "other/y.txt",
	"proj/out.txt":   "outside/z.txt",
	"proj/dout":      "outside",
	"proj/gone.txt":  "proj/nothing.txt",
	"proj/loop1.txt": "proj/loop2.txt",
	"proj/loop2.txt": "proj/loop1.txt",
}

// singleLinkCase is one load.text path and its text when read, else the E7004 cause it is refused with.
type singleLinkCase struct {
	name, path, text string
	read             bool
	refused          diag.Kind
}

// WIRE.md §6.1, DECISIONS 420: single-file link bounds on the real disk, the project under a linked ancestor.
func TestSingleFileLinkBounds(t *testing.T) {
	skipWithoutSymlinks(t)
	root := t.TempDir()
	mkTree(t, filepath.Join(root, "real"), singleLinkFiles, singleLinkLinks)
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(filepath.Join(root, "real"), alias); err != nil {
		t.Fatal(err)
	}
	layout := layoutAt(t, filepath.ToSlash(filepath.Join(alias, "proj")), project.Root{Name: "other", Path: "../other"})
	cases := []singleLinkCase{
		{"through an ancestor of the project root", "plain.txt", "plain", true, 0},
		{"link inside the project", "in.txt", "inside", true, 0},
		{"link into another root", "root.txt", "other", true, 0},
		{"link outside", "out.txt", "", false, diag.KindReadOutsideRoots},
		{"through a directory link outside", "dout/z.txt", "", false, diag.KindReadOutsideRoots},
		{"dangling link", "gone.txt", "", false, diag.KindReadMissing},
		{"loop", "loop1.txt", "", false, diag.KindReadLinkLoop},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { checkSingleText(t, project.OS(), layout, c) })
	}
}

// checkSingleText runs `load.text(c.path)` over fsys, placed by layout: c.text read, or one E7004 c.refused.
func checkSingleText(t *testing.T, fsys project.FS, layout *project.Layout, c singleLinkCase) {
	t.Helper()
	set := &source.FileSet{}
	bag := diag.NewBag(set, "p")
	l := &load.Loader{FS: fsys, Layout: layout, Set: set}
	v, ok, err := l.Load(context.Background(), load.Request{Pkg: "p", Bag: bag}, textExpr(c.path), types.StringType)
	if err != nil {
		t.Fatal(err)
	}
	var msgs []string
	for _, f := range bag.Findings() {
		msgs = append(msgs, f.Message)
	}
	if c.read {
		if !ok || len(msgs) != 0 || v.(*value.Str).V != c.text {
			t.Errorf("ok=%v value=%v findings=%q, want %q read", ok, v, msgs, c.text)
		}
		return
	}
	want := []string{messageOf(diag.E7004.At(source.Span{}, c.path, c.refused))}
	if ok || !slices.Equal(msgs, want) {
		t.Errorf("ok=%v findings=%q, want %q", ok, msgs, want)
	}
}

// deniedLinks is memFS whose link resolution is refused, as a junction whose Readlink is denied.
type deniedLinks struct{ memFS }

func (deniedLinks) EvalSymlinks(string) (string, error) { return "", fs.ErrPermission }

// DECISIONS 420, log-2026-10-31 LOAD-LINKS ruling: a failed resolution refuses the file; only an FS without links lets the stat decide.
func TestSingleFileLinkResolveFails(t *testing.T) {
	files := newMemFS(&txtar.Archive{Files: []txtar.File{{Name: "a.txt", Data: []byte("a")}}})
	layout := layoutAt(t, projectDir)
	t.Run("resolution denied", func(t *testing.T) {
		checkSingleText(t, deniedLinks{files}, layout, singleLinkCase{"", "a.txt", "", false, diag.KindReadPermission})
	})
	t.Run("no links", func(t *testing.T) {
		checkSingleText(t, noLinks{files}, layout, singleLinkCase{"", "a.txt", "a", true, 0})
	})
}
