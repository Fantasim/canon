package build

import (
	"testing"

	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/testkit/winpaths"
)

// API.md §2.2: a project directory written with backslashes or a lowercase drive opens the project the canonical name does.
func TestOpenDirOnWindows(t *testing.T) {
	win := project.NewPaths(winpaths.Sep, winpaths.VolumeName)
	fsys := roFS{"D:/law/project.canon": srcFile("project acme {\n  canon: \"0.1\"\n}\n")}
	for _, dir := range []string{"D:/law", `D:\law`, `d:\law\`, `d:/law/./x/..`} {
		p, err := openIn(win, fsys, dir, Options{})
		if err != nil || p.Dir() != "D:/law" {
			t.Errorf("openIn(%q) = %v, %v; want the project at D:/law", dir, p, err)
		}
	}
}
