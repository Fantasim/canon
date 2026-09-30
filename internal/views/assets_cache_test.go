package views_test

import (
	"context"
	"maps"
	"path"
	"runtime"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/views/encode"
	"github.com/fantasim/canonlang/internal/views/shape"
)

// assetSite is one asset type of a program, with what a fresh walk of every node resolves for it.
type assetSite struct {
	file    string
	spec    *types.AssetSpec
	display string
	abs     string
	found   bool
}

// walked resolves every asset type of prog by a walk of every node of every file, as NewAssets
// did before it remembered the asset types of a file.
func walked(prog *check.Program, layout *project.Layout) []assetSite {
	var out []assetSite
	for _, p := range prog.Packages {
		for _, f := range p.Files {
			syntax.Inspect(f, func(n syntax.Node) bool {
				at, ok := n.(*syntax.AssetType)
				if !ok || prog.Info.TypeExprs[at] == nil {
					return true
				}
				l := shape.LayersOf(prog.Info.TypeExprs[at])
				r, found := layout.Resolve(l.Asset.Root, path.Dir(f.Src.Path), f.Span(at), diag.NewBag(nil, ""))
				out = append(out, assetSite{file: f.Src.Path, spec: l.Asset, display: r.Display, abs: r.Abs, found: found})
				return true
			})
		}
	}
	return out
}

// filesOf is the files of prog by path.
func filesOf(prog *check.Program) map[string]*syntax.File {
	out := map[string]*syntax.File{}
	for _, p := range prog.Packages {
		for _, f := range p.Files {
			out[f.Src.Path] = f
		}
	}
	return out
}

// listed is what NewAssets gives every asset type of prog, in walk order: the file, the root and
// the directory of its root.
func listed(prog *check.Program, layout *project.Layout) []string {
	got := encode.NewAssets(prog, layout)
	var out []string
	for _, s := range walked(prog, layout) {
		dir, _ := got.Dir(s.display)
		out = append(out, s.file+" "+got.Root(s.spec)+" "+dir)
	}
	return out
}

// sameAssets fails unless NewAssets gives every asset type of prog the root and directory of a
// fresh walk, and those a cold analysis of the same tree gives.
func sameAssets(t *testing.T, prog, cold *check.Program, layout *project.Layout, want []string) {
	t.Helper()
	sites := walked(prog, layout)
	var shown []string
	for _, s := range sites {
		shown = append(shown, s.display)
	}
	if !slices.Equal(shown, want) {
		t.Fatalf("asset roots %q, want %q", shown, want)
	}
	got := encode.NewAssets(prog, layout)
	for _, s := range sites {
		if root := got.Root(s.spec); root != s.display {
			t.Errorf("%s: root %q, a fresh walk %q", s.file, root, s.display)
		}
		dir, ok := got.Dir(s.display)
		if ok != s.found || (ok && dir != s.display) {
			t.Errorf("%s: directory %q (%v) of %q, a fresh walk found %v", s.file, dir, ok, s.display, s.found)
		}
	}
	if a, c := listed(prog, layout), listed(cold, layout); !slices.Equal(a, c) {
		t.Errorf("incremental %q, cold %q", a, c)
	}
}

// coldProgram analyzes a copy of fsys with a project of its own, nothing reused.
func coldProgram(t *testing.T, fsys mapFS) *check.Program {
	t.Helper()
	p, err := build.Open(maps.Clone(fsys), lawDir, build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	a, err := p.Analyze(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	return a.Program()
}

// IMPLEMENTATION-PLAN 7.6, VIEWMODEL.md 12.3 `asset.root`: a cached analysis lists the asset roots
// as a fresh walk and a cold analysis do, through an edit, an add, a removal and a collection.
func TestAssetRootsIncrementalEqualCold(t *testing.T) {
	src := func(name, root string) *fstest.MapFile {
		return &fstest.MapFile{Data: []byte("package a\n\nrecord " + name + " {\n  icon: asset(\"" + root + "\", ext: [png])\n}\n")}
	}
	fsys := mapFS{
		"law/project.canon": {Data: []byte("project demo {\n  canon: \"0.1\"\n}\n")},
		"law/a/a.canon":     src("Top", "icons"),
		"law/a/sub/b.canon": src("Deep", "icons"),
	}
	p, err := build.Open(fsys, lawDir, build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	p = p.WithCache(build.NewCache())
	layout, _ := project.NewLayout(loadProject(t, fsys, lawDir), lawDir, nil, diag.NewBag(nil, ""))
	const fileA, fileB = "a/a.canon", "a/sub/b.canon"
	steps := []struct {
		name    string
		apply   func()
		want    []string
		kept    []string // the files that must be those of the step before
		changed []string // the files that must be parsed anew
	}{
		{"first", func() {}, []string{"a/icons", "a/sub/icons"}, nil, nil},
		{"unchanged", func() {}, []string{"a/icons", "a/sub/icons"}, []string{fileA, fileB}, nil},
		{"edited", func() { fsys["law/a/sub/b.canon"] = src("Deep", "pictures") }, []string{"a/icons", "a/sub/pictures"}, []string{fileA}, []string{fileB}},
		{"added", func() { fsys["law/a/c.canon"] = src("Extra", "sounds") }, []string{"a/icons", "a/sounds", "a/sub/pictures"}, []string{fileA, fileB}, nil},
		{"removed", func() { delete(fsys, "law/a/sub/b.canon") }, []string{"a/icons", "a/sounds"}, []string{fileA}, nil},
		{"collected", func() { runtime.GC() }, []string{"a/icons", "a/sounds"}, []string{fileA}, nil},
	}
	var before map[string]*syntax.File
	for _, s := range steps {
		s.apply()
		a, err := p.Analyze(context.Background(), nil)
		if err != nil {
			t.Fatalf("%s: %v", s.name, err)
		}
		files := filesOf(a.Program())
		t.Run(s.name, func(t *testing.T) {
			for _, k := range s.kept {
				if files[k] == nil || files[k] != before[k] {
					t.Errorf("%s was parsed again", k)
				}
			}
			for _, k := range s.changed {
				if files[k] == nil || files[k] == before[k] {
					t.Errorf("%s was not parsed anew", k)
				}
			}
			sameAssets(t, a.Program(), coldProgram(t, fsys), layout, s.want)
		})
		before = files
	}
}
