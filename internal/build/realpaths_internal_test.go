package build

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/fantasim/canonlang/internal/project"
)

// log-2026-09-29 M4 P14-r3, P14-r4: a name is its real path through the links, an absolute target
// too; a dangling link, the name its text leads to; a link in a cycle, itself; a name in a
// directory not there yet, its name below the real path of the first directory that is.
func TestRealPaths(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "a"), dirMode); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "b"), dirMode); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b", "b.json"), []byte("1\n"), fileMode); err != nil {
		t.Fatal(err)
	}
	links := map[string]string{
		"a/data.json": "../b/b.json", "a/gone.json": "../b/new.json", "alias": "b",
		"a/loop1": "loop2", "a/loop2": "loop1", "a/abs.json": filepath.Join(dir, "b", "b.json"),
	}
	//canon:unordered each link is made alone
	for link, target := range links {
		if err := os.Symlink(target, filepath.Join(dir, filepath.FromSlash(link))); err != nil {
			t.Skipf("no symbolic links here: %v", err)
		}
	}
	root, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	base, real := filepath.ToSlash(dir), filepath.ToSlash(root)
	r := newRealPaths(OS())
	cases := map[string]string{
		"a/data.json":    "b/b.json",
		"a/gone.json":    "b/new.json",
		"alias/b.json":   "b/b.json",
		"alias/x/y.json": "b/x/y.json",
		"a/loop1":        "a/loop1",
		"a/abs.json":     "b/b.json",
	}
	//canon:unordered each name is resolved alone
	for name, want := range cases {
		if got := r.file(project.Join(base, name)); got != project.Join(real, want) {
			t.Errorf("%s resolves to %s, want %s", name, got, project.Join(real, want))
		}
	}
}
