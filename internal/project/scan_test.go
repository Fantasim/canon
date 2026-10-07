package project_test

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/fantasim/canonlang/internal/project"
)

// API.md O2 (DECISIONS 332): IsSource is the scan's own choice of source files: every .canon
// file, dot-prefixed ones included, but the top project.canon and project.local.canon and
// anything that is not a .canon file; elsewhere, a file of either name is a source.
func TestIsSource(t *testing.T) {
	tree := fstest.MapFS{"d": {Mode: fs.ModeDir}, "f": {}}
	dir, _ := fs.ReadDir(tree, ".")
	for _, c := range []struct {
		name string
		e    fs.DirEntry
		want bool
	}{
		{"a/a.canon", dir[1], true},
		{"a/.hidden.canon", dir[1], true},
		{"a/project.canon", dir[1], true},
		{"project.canon", dir[1], false},
		{"project.local.canon", dir[1], false},
		{"a/project.local.canon", dir[1], true},
		{"a/a.canon.swp", dir[1], false},
		{"a/x.canon", dir[0], false},
	} {
		if got := project.IsSource(c.name, c.e); got != c.want {
			t.Errorf("IsSource(%s) = %v, want %v", c.name, got, c.want)
		}
	}
}
