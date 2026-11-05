package build_test

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// walkSource holds a load.dir of each kind: a glob with literal segments, one with none (the
// file's own directory), a literal path and a single-file load, which start no walk.
const walkSource = "/// A.\npackage a\n\n/// T.\nrecord T {\n  /// V.\n  v: Int = 1\n}\n\n/// Box.\nrecord Box {\n" +
	"  /// Gs.\n  gs: [T] = []\n}\n\n/// The box.\nlet box: Box = {}\n\n/// Ts.\nlet ts: table T = load.dir(\"items/*.json\")\n\n" +
	"/// Us.\nlet us: [T] = load.dir(\"**/*.json\")\n\n/// Lit.\nlet lit: [T] = load.dir(\"lit/x.json\")\n\n" +
	"/// One.\nlet one: T = load(\"one.json\")\n"

// walkAnalysis analyzes the project of fsys at dir.
func walkAnalysis(t *testing.T, fsys project.FS, dir string) *build.Analysis {
	t.Helper()
	p, err := build.Open(fsys, dir, build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	a, err := p.Analyze(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// API.md N6 (DECISIONS 482), WIRE.md 6.5: every load.dir's base, a layer's included, with its file;
// no wildcard or a single-file load starts no walk, no literal segment walks the file's own
// directory; FileWalkBases judges a file parsed outside the snapshot by the same rule.
func TestWalkBases(t *testing.T) {
	a := walkAnalysis(t, mapFS{
		"p/project.canon":       file("project acme {\n  canon: \"0.1\"\n}\n"),
		"p/a/a.canon":           file(walkSource),
		"p/a/dev.layer.canon":   file("package a\nlayer dev\n\namend box {\n  gs: load.dir(\"../l/*.json\")\n}\n"),
		"p/a/items/one.json":    file("{}\n"),
		"p/l/x.json":            file("{}\n"),
		"p/a/one.json":          file("{}\n"),
		"p/a/lit/x.json":        file("{}\n"),
		"p/a/items/sub/no.text": file("x\n"),
	}, "/p")
	want := []build.WalkBase{{File: "a/a.canon", Dir: "/p/a/items"}, {File: "a/a.canon", Dir: "/p/a"}, {File: "a/dev.layer.canon", Dir: "/p/l"}}
	if got := a.WalkBases(); !slices.Equal(got, want) {
		t.Errorf("WalkBases = %v, want %v", got, want)
	}
	var set source.FileSet
	src, err := set.Add("a/new.canon", "/p/a/new.canon", []byte("package a\n\nlet n: [Int] = load.dir(\"../l/*/*.json\")\nlet m: [Int] = load.dir(\"x.json\")\n"))
	if err != nil {
		t.Fatal(err)
	}
	f := syntax.Parse(src, syntax.FileSource, diag.NewBag(&set, ""))
	if got := a.FileWalkBases(f); !slices.Equal(got, []string{"/p/l"}) {
		t.Errorf("FileWalkBases = %v, want [/p/l]", got)
	}
}

// API.md N6 (DECISIONS 482), WIRE.md 6.5: a base that is a link is named as written, not
// followed; the edit compares bases by real path itself.
func TestWalkBasesLink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need elevated privilege on windows")
	}
	proj := filepath.Join(t.TempDir(), "proj")
	for name, data := range map[string]string{ //canon:unordered each file is written alone
		"project.canon": "project acme {\n  canon: \"0.1\"\n}\n",
		"h/h.canon":     "/// H.\npackage h\n\n/// Hs.\nlet hs: [Int] = load.dir(\"bl/*.json\")\n",
		"e/x.json":      "1\n",
	} {
		p := filepath.Join(proj, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink("../e", filepath.Join(proj, "h", "bl")); err != nil {
		t.Skipf("no symbolic links here: %v", err)
	}
	dir := filepath.ToSlash(proj)
	want := []build.WalkBase{{File: "h/h.canon", Dir: dir + "/h/bl"}}
	if got := walkAnalysis(t, build.OS(), dir).WalkBases(); !slices.Equal(got, want) {
		t.Errorf("WalkBases = %v, want %v", got, want)
	}
}
