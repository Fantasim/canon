package build

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
)

// treeFS is a fstest.MapFS asked for its names with a leading "/".
type treeFS struct{ m fstest.MapFS }

func (t treeFS) ReadFile(name string) ([]byte, error)       { return t.m.ReadFile(name[1:]) }
func (t treeFS) Stat(name string) (fs.FileInfo, error)      { return t.m.Stat(name[1:]) }
func (t treeFS) ReadDir(name string) ([]fs.DirEntry, error) { return t.m.ReadDir(name[1:]) }

// listLog is a project.FS that notes every name it is asked for.
type listLog struct {
	project.FS
	asked []string
}

func (l *listLog) ReadDir(name string) ([]fs.DirEntry, error) {
	l.asked = append(l.asked, name)
	return l.FS.ReadDir(name)
}

func (l *listLog) Stat(name string) (fs.FileInfo, error) {
	l.asked = append(l.asked, name)
	return l.FS.Stat(name)
}

// WIRE.md §2.2 rule 5: an absent optional root is told apart before any listing; a present one is listed.
func TestAssetsAbsentRoot(t *testing.T) {
	tree := treeFS{fstest.MapFS{"here/Icon/a.png": &fstest.MapFile{}}}
	p := &project.Project{Roots: []project.Root{
		{Name: "gone", Path: "../gone", Optional: true},
		{Name: "here", Path: "../here", Optional: true},
		{Name: "own", Path: "own"},
	}}
	layout, ok := project.Place(p, "/p", project.Placement{FS: tree}, diag.NewBag(nil, ""))
	if !ok {
		t.Fatal("layout")
	}
	log := &listLog{FS: tree}
	a := &assets{fs: log, layout: layout, host: &evalHost{}, dirs: map[string]dirListing{}}
	for _, c := range []struct {
		root   string
		name   string
		absent bool
	}{
		{"@gone/Icon", "gone", true},
		{"@here/Icon", "", false},
		{"@own/Icon", "", false},
		{"Icon", "", false}, // unrooted
	} {
		if name, absent := a.Absent(c.root, "items"); name != c.name || absent != c.absent {
			t.Errorf("Absent(%s) = %q, %v; want %q, %v", c.root, name, absent, c.name, c.absent)
		}
	}
	if len(log.asked) != 0 {
		t.Errorf("Absent asked the file system for %v", log.asked)
	}
	if _, found := a.Exists("@here/Icon", "items", "a.png"); !found {
		t.Error("a present optional root is listed as any root is")
	}
}
