package build

import (
	"errors"
	"io/fs"
	"runtime"
	"testing"
	"testing/fstest"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
)

const linkOutput = "/o/out/Evil.png/x.txt"

// deniedList is a file system whose listing of one directory fails with a permission error.
type deniedList struct {
	roFS
	dir string
}

func (d deniedList) ReadDir(name string) ([]fs.DirEntry, error) {
	if name == d.dir {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrPermission}
	}
	return d.roFS.ReadDir(name)
}

// linkRun is a run of a project at /o over fsys with one bag, for package a.
func linkRun(fsys project.FS) *run {
	s := placeSnapshot()
	s.p, s.set, s.bags = &Project{fs: fsys, dir: "/o"}, &source.FileSet{}, map[string]*diag.Bag{}
	return &run{p: s.p, s: s}
}

func linkOutputs() []*output {
	return []*output{{Output: Output{Path: "out/Evil.png/x.txt", Abs: linkOutput, Package: "a"}}}
}

// API.md B1d, DECISIONS 342 (settled 2026-10-09): a directory that cannot be listed refuses the
// build with its I/O error; it never counts as holding no link.
func TestThroughLinksFailsClosed(t *testing.T) {
	r := linkRun(deniedList{roFS: roFS{}, dir: "/o/out"})
	linked, err := r.throughLinks(linkOutputs())
	if linked || !errors.Is(err, fs.ErrPermission) {
		t.Errorf("listing denied: linked %v, %v", linked, err)
	}
}

// API.md B1d, DECISIONS 342 (settled 2026-10-09): a link's name matches whatever its letter
// case, as a case-insensitive disk resolves it; a directory that does not exist holds no link.
func TestThroughLinksIgnoresCase(t *testing.T) {
	r := linkRun(roFS{"o/out/EVIL.PNG": &fstest.MapFile{Data: []byte("/elsewhere"), Mode: fs.ModeSymlink}})
	linked, err := r.throughLinks(linkOutputs())
	if !linked || err != nil {
		t.Errorf("EVIL.PNG for Evil.png: linked %v, %v", linked, err)
	}
	clean := linkRun(roFS{})
	if linked, err := clean.throughLinks(linkOutputs()); linked || err != nil {
		t.Errorf("nothing on disk: linked %v, %v", linked, err)
	}
}

// API.md B1d, DECISIONS 342 (settled 2026-10-09): a symbolic link is a link everywhere, and a
// reparse point Go lists as irregular (a Windows junction) is one on Windows.
func TestLinkEntry(t *testing.T) {
	for _, c := range []struct {
		mode fs.FileMode
		want bool
	}{
		{fs.ModeSymlink, true},
		{fs.ModeDir, false},
		{0, false},
		{fs.ModeIrregular, runtime.GOOS == "windows"},
	} {
		if got := linkEntry(c.mode); got != c.want {
			t.Errorf("linkEntry(%v) = %v, want %v", c.mode, got, c.want)
		}
	}
}
