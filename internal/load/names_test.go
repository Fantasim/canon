package load_test

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/load"
	"github.com/fantasim/canonlang/internal/project"
)

// API.md S3, S5, E17 (DECISIONS 330): a load names, from its literal path alone, the files the
// loader's own walk matches and the directories it lists, or its one path, missing or not; a raw
// string is a literal path, any other path names nothing (E7008).
func TestNames(t *testing.T) {
	skipOnWindows(t)
	root := filepath.ToSlash(t.TempDir())
	mkTree(t, root, map[string]string{
		"proj/d/a1.json": "{}", "proj/d/sub/c.json": "{}", "proj/d/.h.json": "{}", "proj/real/x.json": "{}", "outside/o.json": "{}",
	}, map[string]string{"proj/d/in.json": "proj/real/x.json", "proj/d/out.json": "outside/o.json"})
	proj := root + "/proj"
	layout := layoutAt(t, proj)
	realProj, err := filepath.EvalSymlinks(proj) // the walk lists real paths (macOS: /var is /private/var)
	if err != nil {
		t.Fatal(err)
	}
	realProj = filepath.ToSlash(realProj)
	for _, c := range []struct {
		name, call string
		files      []string
		literal    bool
	}{
		{"deep glob", `load.dir("../d/**/*.json")`, []string{"d/a1.json", "d/in.json", "d/sub/c.json"}, true},
		{"missing", `load("../d/none.json")`, []string{"d/none.json"}, true},
		{"defines", `load.defines("../d/h.h")`, []string{"d/h.h"}, true},
		{"raw", `load.text(r"../d/a1.json")`, []string{"d/a1.json"}, true},
		{"leaves the project", `load("../../x.json")`, nil, true},
		{"interpolated", `load("../d/{N}.json")`, nil, false},
		{"named", `load(P)`, nil, false},
	} {
		e := callOf(t, c.call)
		named, ok := load.Names(project.OS(), layout, "a", e)
		var got []string
		for _, f := range named.Files {
			got = append(got, f.Display)
		}
		if ok != c.literal || !slices.Equal(got, c.files) {
			t.Errorf("%s: Names = %q, %v; want %q, %v", c.name, got, ok, c.files, c.literal)
		}
		if e.Method != nil && e.Method.Name == "dir" {
			same := load.Files(project.OS(), layout, "a", e, diag.NewBag(nil, ""))
			if !slices.Equal(named.Files, same) {
				t.Errorf("%s: Names %v, the loader's matches %v", c.name, named.Files, same)
			}
			if !slices.Contains(named.Dirs, realProj+"/d/sub") || len(named.Links) == 0 {
				t.Errorf("%s: the walk listed %v and resolved %v", c.name, named.Dirs, named.Links)
			}
		}
	}
}
