package progen

import (
	"io/fs"
	"path/filepath"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/testkit/winpaths"
)

// TestReadOnlyVolumes: a name with a Windows volume reads the "/p" tree (API.md §2.2).
func TestReadOnlyVolumes(t *testing.T) {
	const manifest = "project p {}\n"
	entries := []string{"b.canon", "l.canon"}
	p := NewProject()
	p.Set("project.canon", []byte(manifest))
	p.Set("a/b.canon", []byte("package a\n"))
	p.Link("a/l.canon", "b.canon")
	cases := []struct {
		name   string
		volume func(string) string
		dir    string
	}{
		{"drive", winpaths.VolumeName, "D:/p"},
		{"unc", winpaths.VolumeName, "//server/share/p"},
		{"host", filepath.VolumeName, projectDir},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := readOnly{m: p.fsys(), volume: tc.volume}
			if data, err := r.ReadFile(tc.dir + "/project.canon"); err != nil || string(data) != manifest {
				t.Errorf("ReadFile: %q, %v", data, err)
			}
			if fi, err := r.Stat(tc.dir + "/a"); err != nil || !fi.IsDir() {
				t.Errorf("Stat: %v", err)
			}
			es, err := r.ReadDir(tc.dir + "/a")
			if err != nil || !slices.Equal(names(es), entries) {
				t.Errorf("ReadDir: %v, %v, want %v", names(es), err, entries)
			}
			if got, err := r.EvalSymlinks(tc.dir + "/a/l.canon"); err != nil || got != tc.dir+"/a/b.canon" {
				t.Errorf("EvalSymlinks: %q, %v", got, err)
			}
		})
	}
}

func names(es []fs.DirEntry) []string {
	out := make([]string, 0, len(es))
	for _, e := range es {
		out = append(out, e.Name())
	}
	return out
}
