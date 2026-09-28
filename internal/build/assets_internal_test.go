package build

import (
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
)

// ciDir is a directory tree keyed by each entry's true-case name; a nil value is a file, a
// non-nil one a subdirectory.
type ciDir map[string]ciDir

// ciFS is a project.FS whose ReadDir folds case, standing in for a case-insensitive disk (TYPES.md §13.4).
type ciFS ciDir

func (fsys ciFS) ReadFile(string) ([]byte, error)  { return nil, fs.ErrNotExist }
func (fsys ciFS) Stat(string) (fs.FileInfo, error) { return nil, fs.ErrNotExist }

// ReadDir lists dir's own entries, resolving every "/"-separated segment of name by fold.
func (fsys ciFS) ReadDir(name string) ([]fs.DirEntry, error) {
	dir, ok := ciDir(fsys).lookup(name)
	if !ok {
		return nil, fs.ErrNotExist
	}
	entries := make([]fs.DirEntry, 0, len(dir))
	for _, n := range slices.Sorted(maps.Keys(dir)) {
		entries = append(entries, ciEntry{name: n, dir: dir[n] != nil})
	}
	return entries, nil
}

// lookup walks name from d, one segment at a time, each matched by letter-case fold.
func (d ciDir) lookup(name string) (ciDir, bool) {
	cur := d
	for _, seg := range strings.Split(strings.Trim(name, pathSep), pathSep) {
		if seg == "" || seg == "." {
			continue
		}
		next, ok := cur.fold(seg)
		if !ok || next == nil {
			return nil, false
		}
		cur = next
	}
	return cur, true
}

// fold finds seg among d's own entries, matched without regard to letter case.
func (d ciDir) fold(seg string) (ciDir, bool) {
	for n, sub := range d { //canon:unordered a fold match does not depend on which name is seen first
		if strings.EqualFold(n, seg) {
			return sub, true
		}
	}
	return nil, false
}

// ciEntry is one entry of a ciFS listing, its true case kept.
type ciEntry struct {
	name string
	dir  bool
}

func (e ciEntry) Name() string             { return e.name }
func (e ciEntry) IsDir() bool              { return e.dir }
func (ciEntry) Type() fs.FileMode          { return 0 }
func (ciEntry) Info() (fs.FileInfo, error) { return nil, nil }

// TYPES.md §13.4: a folder segment that differs from the disk only by letter case is missing.
func TestAssetExistsFolderCase(t *testing.T) {
	tree := ciDir{"assets": ciDir{"Icons": ciDir{"x.png": nil}}}
	p := &project.Project{Roots: []project.Root{{Name: "assets", Path: "assets"}}}
	layout, ok := project.NewLayout(p, "/", nil, diag.NewBag(nil, ""))
	if !ok {
		t.Fatal("layout")
	}
	a := &assets{fs: ciFS(tree), layout: layout, host: &evalHost{}, dirs: map[string]dirListing{}}

	for _, c := range []struct {
		name, path string
		found      bool
	}{
		{"exact case", "Icons/x.png", true},
		{"folder case mismatch", "icons/x.png", false},
		{"file case mismatch", "Icons/X.png", false},
	} {
		if _, found := a.Exists("@assets", "items", c.path); found != c.found {
			t.Errorf("%s: Exists(%q) = %v, want %v", c.name, c.path, found, c.found)
		}
	}
}

// A symbolic link is followed to its target, as the real Resource tree needs (regression: a
// folder listed by fs.DirEntry.IsDir(), which is false for a link, must not drop it from subs).
func TestAssetExistsSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need elevated privilege on windows")
	}
	root := t.TempDir()
	mustMkdir(t, filepath.Join(root, "target"))
	mustWrite(t, filepath.Join(root, "target", "x.png"), "x")
	mustWrite(t, filepath.Join(root, "real.png"), "y")
	mustSymlink(t, filepath.Join(root, "target"), filepath.Join(root, "linkdir"))
	mustSymlink(t, filepath.Join(root, "real.png"), filepath.Join(root, "linkfile.png"))

	p := &project.Project{Roots: []project.Root{{Name: "assets", Path: "."}}}
	layout, ok := project.NewLayout(p, filepath.ToSlash(root), nil, diag.NewBag(nil, ""))
	if !ok {
		t.Fatal("layout")
	}
	a := &assets{fs: project.OS(), layout: layout, host: &evalHost{}, dirs: map[string]dirListing{}}

	for _, c := range []struct{ name, path string }{
		{"asset under a symlinked folder", "linkdir/x.png"},
		{"symlinked file", "linkfile.png"},
	} {
		if _, found := a.Exists("@assets", "items", c.path); !found {
			t.Errorf("%s: Exists(%q) = false, want true", c.name, c.path)
		}
	}
}

func mustMkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func mustSymlink(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
}
