package load_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/load"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"golang.org/x/tools/txtar"
)

// linkRun is one `load.dir` run's result: the keyed list's ids, the display paths of the files
// read, and whether W7115 was reported.
type linkRun struct {
	ids, read []string
	w7115     bool
}

// runKeyed runs `load.dir(pattern)` as `[Item] keyed by id` over fsys, placed by layout.
func runKeyed(t *testing.T, fsys project.FS, layout *project.Layout, pattern string) linkRun {
	t.Helper()
	set := &source.FileSet{}
	bag := diag.NewBag(set, "p")
	l := &load.Loader{FS: fsys, Layout: layout, Set: set}
	item := itemType()
	keyed := &types.ListType{Elem: item, KeyedBy: item.Fields[0]}
	v, ok, err := l.Load(context.Background(), load.Request{Pkg: "p", Bag: bag}, dirExpr(pattern), keyed)
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v findings %+v", ok, err, bag.Findings())
	}
	var run linkRun
	for _, e := range v.(*value.List).Elems {
		run.ids = append(run.ids, e.(*value.Record).Ident.Key.S)
	}
	for id := source.FileID(1); set.File(id) != nil; id++ {
		run.read = append(run.read, set.Path(id))
	}
	for _, f := range bag.Findings() {
		run.w7115 = run.w7115 || f.Code == diag.W7115.Def().Code
	}
	return run
}

// layoutAt is a Layout for a project at dir with the given roots.
func layoutAt(t *testing.T, dir string, roots ...project.Root) *project.Layout {
	t.Helper()
	layout, ok := project.NewLayout(&project.Project{Roots: roots}, dir, nil, diag.NewBag(nil, ""))
	if !ok {
		t.Fatal("layout")
	}
	return layout
}

// mkTree writes files (content by '/'-separated path under root) and links (target by link
// path, targets absolute) on the real disk.
func mkTree(t *testing.T, root string, files, links map[string]string) {
	t.Helper()
	for name, content := range files {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	for name, target := range links {
		p := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.Symlink(filepath.Join(root, filepath.FromSlash(target)), p); err != nil {
			t.Fatal(err)
		}
	}
}

func skipOnWindows(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need elevated privilege on windows")
	}
}

// symlinkFiles is the tree of the real-disk symlink tests: two files in the project, one outside.
var symlinkFiles = map[string]string{
	"proj/data/a.json":      `{"id": "a", "name": "A"}`,
	"proj/data/real/b.json": `{"id": "b", "name": "B"}`,
	"outside/escape.json":   `{"id": "e", "name": "E"}`,
}

// symlinkLinks are its links: a directory link inside the project, a file link outside it, and
// a directory link back to the walk's own base.
var symlinkLinks = map[string]string{
	"proj/data/gooddir":      "proj/data/real",
	"proj/data/badlink.json": "outside/escape.json",
	"proj/data/loop":         "proj/data",
}

// WIRE.md §6.5, §2.3: links in bounds followed, outside W7115, loops cut, one file kept once.
func TestLoadDirSymlinks(t *testing.T) {
	skipOnWindows(t)
	root := t.TempDir()
	mkTree(t, root, symlinkFiles, symlinkLinks)
	run := runKeyed(t, project.OS(), layoutAt(t, filepath.ToSlash(filepath.Join(root, "proj"))), "data/**/*.json")
	if want := []string{"a", "b"}; !slices.Equal(run.ids, want) {
		t.Errorf("entries = %v, want %v", run.ids, want)
	}
	if want := []string{"data/a.json", "data/gooddir/b.json"}; !slices.Equal(run.read, want) {
		t.Errorf("files read = %v, want %v", run.read, want)
	}
	if !run.w7115 {
		t.Errorf("no %s for the out-of-bounds link", diag.W7115.Def().Code)
	}
}

// "load.dir round 3", review round 2 R2-2: a project reached through a link (macOS's /tmp) is
// bounded by its resolved directory, as targets are, so its own links are followed, not W7115.
func TestLoadDirSymlinksThroughAlias(t *testing.T) {
	skipOnWindows(t)
	root := t.TempDir()
	mkTree(t, filepath.Join(root, "real"), symlinkFiles, symlinkLinks)
	alias := filepath.Join(root, "alias")
	if err := os.Symlink(filepath.Join(root, "real"), alias); err != nil {
		t.Fatal(err)
	}
	run := runKeyed(t, project.OS(), layoutAt(t, filepath.ToSlash(filepath.Join(alias, "proj"))), "data/**/*.json")
	if want := []string{"a", "b"}; !slices.Equal(run.ids, want) {
		t.Errorf("entries through the alias = %v, want %v", run.ids, want)
	}
	if want := []string{"data/a.json", "data/gooddir/b.json"}; !slices.Equal(run.read, want) {
		t.Errorf("files read through the alias = %v, want %v", run.read, want)
	}
}

// meta/decisions/log-2026-09-24.md "load.dir round 2": a link is followed when its target
// resolves inside ANY declared root or the project, not only the base's own root.
func TestLoadDirSymlinksAcrossRoots(t *testing.T) {
	skipOnWindows(t)
	root := t.TempDir()
	mkTree(t, root, map[string]string{
		"rootA/data/a.json": `{"id": "a", "name": "A"}`,
		"rootB/b.json":      `{"id": "b", "name": "B"}`,
		"neither/x.json":    `{"id": "x", "name": "X"}`,
	}, map[string]string{
		"rootA/data/link.json": "rootB/b.json",
		"rootA/data/bad.json":  "neither/x.json",
	})
	layout := layoutAt(t, filepath.ToSlash(filepath.Join(root, "proj")),
		project.Root{Name: "A", Path: "../rootA"}, project.Root{Name: "B", Path: "../rootB"})
	run := runKeyed(t, project.OS(), layout, "@A/data/*.json")
	if want := []string{"a", "b"}; !slices.Equal(run.ids, want) {
		t.Errorf("entries = %v, want %v", run.ids, want)
	}
	if !run.w7115 {
		t.Errorf("no %s for the link into neither root", diag.W7115.Def().Code)
	}
}

// linkArchive is an in-memory tree with a directory link and a file link, both in the project.
func linkArchive() *txtar.Archive {
	return &txtar.Archive{Files: []txtar.File{
		{Name: "data/a.json", Data: []byte(`{"id": "a", "name": "A"}`)},
		{Name: "data/real/b.json", Data: []byte(`{"id": "b", "name": "B"}`)},
		{Name: "symlinks", Data: []byte("data/gooddir /p/data/real\ndata/c.json real/b.json\n")},
	}}
}

// "load.dir round 3": link targets are read through the project's FS, never the real disk; an
// in-memory FS resolving its own links follows them, one file reached three ways kept once.
func TestLoadDirSymlinksThroughFS(t *testing.T) {
	run := runKeyed(t, newMemFS(linkArchive()), layoutAt(t, projectDir), "data/**/*.json")
	if want := []string{"a", "b"}; !slices.Equal(run.ids, want) {
		t.Errorf("entries = %v, want %v", run.ids, want)
	}
	if want := []string{"data/a.json", "data/c.json"}; !slices.Equal(run.read, want) {
		t.Errorf("files read = %v, want %v", run.read, want)
	}
	if run.w7115 {
		t.Errorf("unexpected %s", diag.W7115.Def().Code)
	}
}

// noLinks hides memFS's EvalSymlinks: an FS that cannot resolve links.
type noLinks struct{ project.FS }

// "load.dir round 3": an FS without link resolution treats every link as unresolvable, W7115.
func TestLoadDirSymlinksWithoutResolver(t *testing.T) {
	run := runKeyed(t, noLinks{newMemFS(linkArchive())}, layoutAt(t, projectDir), "data/**/*.json")
	if want := []string{"a", "b"}; !slices.Equal(run.ids, want) {
		t.Errorf("entries = %v, want %v", run.ids, want)
	}
	if want := []string{"data/a.json", "data/real/b.json"}; !slices.Equal(run.read, want) {
		t.Errorf("files read = %v, want %v", run.read, want)
	}
	if !run.w7115 {
		t.Errorf("no %s for a link the FS cannot resolve", diag.W7115.Def().Code)
	}
}
