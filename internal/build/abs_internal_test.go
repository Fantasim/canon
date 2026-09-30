package build

import (
	"testing"

	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/testkit/winpaths"
)

// API.md §2.2, §3.4 (log M4 B12-r): Abs on Windows, drive and UNC project directories alike.
func TestAbsOnWindows(t *testing.T) {
	win := project.NewPaths(winpaths.Sep, winpaths.VolumeName)
	cases := []struct {
		dir, file, want string
		ok              bool
	}{
		{"D:/law", "/..", "D:/", true},
		{"D:/law", "/", "D:/", true},
		{"D:/law", "C:/", "C:/", true},
		{"D:/law", "/../x.canon", "D:/x.canon", true},
		{"D:/law", "c:/x/../y.canon", "C:/y.canon", true},
		{"D:/law", "d:/law/b/b.canon", "D:/law/b/b.canon", true},
		{"D:/law", `\law\b\b.canon`, "D:/law/b/b.canon", true},
		{"D:/law", `b\b.canon`, "D:/law/b/b.canon", true},
		{"D:/law", "b/b.canon", "D:/law/b/b.canon", true},
		{"D:/law", `..\x.canon`, "D:/x.canon", false},
		{"D:/law", `\\server\share\proj\a.canon`, "//server/share/proj/a.canon", true},
		{"D:/law", "//server/share/..", "//server/share/", true},
		{"//server/share/proj", "a/a.canon", "//server/share/proj/a/a.canon", true},
		{"//server/share/proj", "/x.canon", "//server/share/x.canon", true},
		{"//wsl$/Ubuntu/home/proj", `a\a.canon`, "//wsl$/Ubuntu/home/proj/a/a.canon", true},
	}
	for _, c := range cases {
		p := &Project{dir: c.dir}
		if got, ok := p.absIn(win, c.file); got != c.want || ok != c.ok {
			t.Errorf("in %s, Abs(%q) = %q, %v; want %q, %v", c.dir, c.file, got, ok, c.want, c.ok)
		}
	}
}

// API.md §2.2 (log M4 B12-r): a rooted path is cleaned without climbing above the root.
func TestAbsRooted(t *testing.T) {
	p := &Project{dir: "/law"}
	for _, c := range [][2]string{{"/..", "/"}, {"/", "/"}, {"/../x.canon", "/x.canon"}, {"/law/./b/b.canon", "/law/b/b.canon"}} {
		if got, ok := p.Abs(c[0]); got != c[1] || !ok {
			t.Errorf("Abs(%q) = %q, %v; want %q", c[0], got, ok, c[1])
		}
	}
}
