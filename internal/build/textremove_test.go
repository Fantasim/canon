package build_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
)

const (
	windows  = "windows"
	rule342  = "CODEGEN.md §2.4, DECISIONS 342: "
	sqlDir   = "p/out/sql"
	showFlag = "SHOW = true"
	hideFlag = "SHOW = false"
)

// removeTree is a project on disk under a temporary directory, built once with the optional files present, and a build function for it.
type removeTree struct {
	t   *testing.T
	tmp string
}

// optionalBothNames are the files optionalSource writes with SHOW true.
var optionalBothNames = []string{"r.json", "s.sql", "t.sql"}

func newRemoveTree(t *testing.T) *removeTree {
	t.Helper()
	tmp := t.TempDir()
	writeTree(t, tmp, map[string]string{"p/project.canon": textProject, "p/a/a.canon": optionalSource})
	rt := &removeTree{t: t, tmp: tmp}
	if _, err := rt.build(false); err != nil {
		t.Fatal(err)
	}
	return rt
}

// build builds the project on the OS file system.
func (rt *removeTree) build(check bool) (*build.BuildResult, error) {
	p, err := build.Open(build.OS(), filepath.ToSlash(filepath.Join(rt.tmp, "p")), build.Options{})
	if err != nil {
		rt.t.Fatal(err)
	}
	return p.Build(context.Background(), build.BuildOptions{Check: check})
}

// hide flips the optional files to none in the source.
func (rt *removeTree) hide() {
	rt.t.Helper()
	src := filepath.Join(rt.tmp, "p", "a", "a.canon")
	if err := os.WriteFile(src, []byte(strings.Replace(optionalSource, showFlag, hideFlag, 1)), 0o600); err != nil {
		rt.t.Fatal(err)
	}
}

// path is a file under the temporary directory.
func (rt *removeTree) path(elem ...string) string {
	return filepath.Join(append([]string{rt.tmp}, elem...)...)
}

// exists reports a file or link, not following a link.
func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// CODEGEN.md §2.9, DECISIONS 336: on the OS file system a none result removes the file from disk.
func TestTextRemovalOnDisk(t *testing.T) {
	rt := newRemoveTree(t)
	rt.hide()
	if _, err := rt.build(false); err != nil {
		t.Fatal(err)
	}
	if exists(rt.path(sqlDir, "s.sql")) || exists(rt.path(sqlDir, "r.json")) || !exists(rt.path(sqlDir, "t.sql")) {
		t.Errorf("%sfiles left: %v", rule336, dirNames(t, rt.path(sqlDir)))
	}
}

// dirNames are the names in dir.
func dirNames(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Name()
	}
	return out
}

// refusedThroughLink reports a build refused with E8027 and nothing written: res holds the finding and every file is as it was (DECISIONS 342).
func refusedThroughLink(res *build.BuildResult, err error) bool {
	return err == nil && slices.Contains(codes(res.List), diag.E8027.Def().Code) && len(res.Outputs) == 0 && len(res.Locks) == 0
}

// DECISIONS 342 (amending 336): removing a listed path that is a symbolic link to a file is E8027; the link and the file it leads to are untouched.
func TestTextRemovalSymlinkToFile(t *testing.T) {
	if runtime.GOOS == windows {
		t.Skip("symlinks need elevated privilege on windows")
	}
	rt := newRemoveTree(t)
	target := rt.path("target.txt")
	if err := os.WriteFile(target, []byte(optionalText), 0o600); err != nil {
		t.Fatal(err)
	}
	link := rt.path(sqlDir, "s.sql")
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("no symbolic links here: %v", err)
	}
	rt.hide()
	if res, err := rt.build(false); !refusedThroughLink(res, err) {
		t.Fatalf("%snot refused: %v, %v", rule342, err, res)
	}
	if got, err := os.ReadFile(target); err != nil || string(got) != optionalText || !exists(link) {
		t.Errorf("%slink %v, target %q, %v", rule342, exists(link), got, err)
	}
}

// DECISIONS 342 (amending 336): removing a listed path whose directory is a symbolic link is E8027, as a write through it is; the link and every file it leads to stay.
func TestTextRemovalThroughDirectoryLink(t *testing.T) {
	if runtime.GOOS == windows {
		t.Skip("symlinks need elevated privilege on windows")
	}
	rt := newRemoveTree(t)
	real := rt.path("real")
	if err := os.Rename(rt.path(sqlDir), real); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(real, rt.path(sqlDir)); err != nil {
		t.Skipf("no symbolic links here: %v", err)
	}
	rt.hide()
	if res, err := rt.build(false); !refusedThroughLink(res, err) {
		t.Fatalf("%snot refused: %v, %v", rule342, err, res)
	}
	if names := dirNames(t, real); !slices.Equal(names, optionalBothNames) || !exists(rt.path(sqlDir)) {
		t.Errorf("%sreal directory: %v", rule342, names)
	}
}

// DECISIONS 336: a listed path that is now a directory is skipped without error, and what it holds stays.
func TestTextRemovalSkipsDirectory(t *testing.T) {
	rt := newRemoveTree(t)
	dir := rt.path(sqlDir, "s.sql")
	if err := os.Remove(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "inner"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	rt.hide()
	res, err := rt.build(false)
	if err != nil || res.Summary.Errors != 0 || !exists(filepath.Join(dir, "inner")) {
		t.Errorf("%sdirectory: %v, %v", rule336, err, res)
	}
}

// CODEGEN.md §2.9, DECISIONS 336: when a removal fails (a read-only directory) the build reports the error like a failed write and leaves everything as it was: the files, canon.outputs, and no temporary file.
func TestTextRemovalFailureIsAllOrNothing(t *testing.T) {
	if runtime.GOOS == windows || os.Geteuid() == 0 {
		t.Skip("directory permissions do not bind here")
	}
	rt := newRemoveTree(t)
	listing := rt.path("p", "a", "canon.outputs")
	before, err := os.ReadFile(listing)
	if err != nil {
		t.Fatal(err)
	}
	rt.hide()
	dir := rt.path(sqlDir)
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) }) // so the temporary directory can be removed
	_, err = rt.build(false)
	if err == nil || !strings.Contains(err.Error(), "@out/sql/") {
		t.Fatalf("%sno error naming the file: %v", rule336, err)
	}
	after, _ := os.ReadFile(listing)
	if string(after) != string(before) || !exists(filepath.Join(dir, "s.sql")) || !exists(filepath.Join(dir, "r.json")) {
		t.Errorf("%shalf done: canon.outputs %q, %v", rule336, after, dirNames(t, dir))
	}
	if names := dirNames(t, dir); len(names) != 3 {
		t.Errorf("%sleftover in the out directory: %v", rule336, names)
	}
	if names := dirNames(t, rt.path("p", "a")); len(names) != 2 {
		t.Errorf("%sleftover in the package directory: %v", rule336, names)
	}
}

// DECISIONS 336: only a regular file is hashed or removed; a listed path that leads to a device (a symbolic link to /dev/zero) is skipped at once, link and device untouched.
func TestTextRemovalSkipsDevices(t *testing.T) {
	if runtime.GOOS == windows {
		t.Skip("no /dev/zero on windows")
	}
	if _, err := os.Stat("/dev/zero"); err != nil {
		t.Skip("no /dev/zero here")
	}
	rt := newRemoveTree(t)
	link := rt.path(sqlDir, "s.sql")
	if err := os.Remove(link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("/dev/zero", link); err != nil {
		t.Skipf("no symbolic links here: %v", err)
	}
	rt.hide()
	res, err := rt.build(false)
	if err != nil || res.Summary.Errors != 0 {
		t.Fatalf("%s%v %v", rule336, err, res)
	}
	if target, err := os.Readlink(link); err != nil || target != "/dev/zero" {
		t.Errorf("%sthe link to a device was touched: %q, %v", rule336, target, err)
	}
}
