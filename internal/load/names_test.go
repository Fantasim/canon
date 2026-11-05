package load_test

import (
	"context"
	"path/filepath"
	"slices"
	"testing"

	"golang.org/x/tools/txtar"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/load"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
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

// WIRE.md 6.5 (LOD-05; API.md N6): DirBase is the directory a load.dir walk starts at, the
// glob's literal segments resolved, whether it exists or not; no other form, no pattern without
// a wildcard, and no path that is no literal or leaves the project has one.
func TestDirBase(t *testing.T) {
	layout := layoutAt(t, projectDir)
	for _, c := range []struct {
		call, want string
		ok         bool
	}{
		{`load.dir("../d/**/*.json")`, "d", true},
		{`load.dir("items/*/*.json")`, "a/items", true},
		{`load.dir("../gone/x/*.json")`, "gone/x", true},
		{`load.dir("./*.json")`, "a", true},
		{`load.dir("*.json")`, "a", true},
		{`load.dir("**/*.json")`, "a", true},
		{`load.dir("../d/x.json")`, "", false},
		{`load("../d/*.json")`, "", false},
		{`load.dir(P)`, "", false},
		{`load.dir("../../x/*.json")`, "", false},
	} {
		got, ok := load.DirBase(layout, "a", callOf(t, c.call))
		if want := project.Join(projectDir, c.want); ok != c.ok || ok && got != want {
			t.Errorf("%s: DirBase = %q, %v; want %q, %v", c.call, got, ok, c.want, c.ok)
		}
	}
}

// WIRE.md 6.5, 2.2 item 1: a pattern with no literal segment walks the directory of the file it
// is written in, so "*.json" reads that directory's files and "**/*.json" every one below it.
func TestDirOwnDirectory(t *testing.T) {
	a := &txtar.Archive{Files: []txtar.File{
		{Name: "a.json", Data: []byte(`{"label": "A"}`)}, {Name: "sub/b.json", Data: []byte(`{"label": "B"}`)},
	}}
	for _, c := range []struct{ pattern, want string }{
		{"*.json", `{a: Row{label: "A"}}`},
		{"**/*.json", `{a: Row{label: "A"}, b: Row{label: "B"}}`},
	} {
		set := &source.FileSet{}
		bag := diag.NewBag(set, "p")
		fsys := newMemFS(a)
		l := &load.Loader{FS: fsys, Layout: machineLayout(t, fsys, bag), Set: set}
		v, ok, err := l.Load(context.Background(), load.Request{Pkg: "p", Bag: bag}, dirExpr(c.pattern), rowType())
		if err != nil || !ok || v == nil || v.CanonText() != c.want || len(bag.Findings()) > 0 {
			t.Errorf("load.dir(%q) = %v, %v, %v, findings %v; want %s", c.pattern, v, ok, err, bag.Findings(), c.want)
		}
	}
}
