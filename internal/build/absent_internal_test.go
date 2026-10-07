package build

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
)

// vanishing is the OS file system whose first write removes the directory gone, as a root
// deleted while a build runs.
type vanishing struct {
	WriteFS
	gone    string
	removed bool
}

func (v *vanishing) WriteFile(name string, data []byte) error {
	if !v.removed {
		v.removed = true
		if err := os.RemoveAll(v.gone); err != nil {
			return err
		}
	}
	return v.WriteFS.WriteFile(name, data)
}

// CODEGEN.md §2.4, DECISIONS 332: a root vanished mid-build is not recreated; the write rolls back.
func TestRootVanishedMidBuild(t *testing.T) {
	tmp := t.TempDir()
	files := map[string]string{
		"p/project.canon": "project acme {\n  canon: \"0.1\"\n  roots {\n    made: \"made\"\n    ext: \"../Ext\"\n  }\n}\n",
		"p/a/a.canon":     "/// A.\npackage a\n\n/// V.\nlet v: Int = 1\n\nemit json { out: [\"@made/\", \"@ext/deep/\"] }\n",
		"Ext/README":      "",
	}
	for name, text := range files { //canon:unordered each file is written under its own name
		abs := filepath.Join(tmp, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(abs), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(abs, []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	ext := filepath.Join(tmp, "Ext")
	p, err := Open(&vanishing{WriteFS: OS(), gone: ext}, filepath.ToSlash(filepath.Join(tmp, "p")), Options{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Build(context.Background(), BuildOptions{}); !errors.Is(err, errRootMissing) {
		t.Fatalf("build: %v", err)
	}
	for _, left := range []string{ext, filepath.Join(tmp, "p", "made")} {
		if _, err := os.Stat(left); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s: %v", left, err)
		}
	}
}

// CODEGEN.md §2.4, DECISIONS 332: never a root's own directory outside the project, nor above it.
func TestDirBounds(t *testing.T) {
	b := &dirBounds{project: "/w/p", roots: []string{"/w/Source", "/w/Source/sov", "/x"}}
	for _, c := range []struct {
		dir  string
		want bool
	}{
		{"/w/p/out", true},
		{"/w/p", true},
		{"/w/Source/gen", true},
		{"/w/Source/sov/gen/deep", true},
		{"/w/Source", false},
		{"/w/Source/sov", false},
		{"/x", false},
		{"/w", false},
		{"/", false},
		{"/w/Sources", false},
	} {
		if got := b.allows(c.dir); got != c.want {
			t.Errorf("allows(%s) = %t, want %t", c.dir, got, c.want)
		}
	}
}

// CODEGEN.md §2.8 "This machine": no relative path between two roots (another volume) is a difference.
func TestCrossRootsNoRelativePath(t *testing.T) {
	proj := &project.Project{Roots: []project.Root{{Name: "x", Path: "x"}}}
	declared, _ := project.NewLayout(proj, "/p", nil, diag.NewBag(nil, ""))
	placed, _ := project.NewLayout(proj, "/p", map[string]string{"x": "Z:/x"}, diag.NewBag(nil, ""))
	if _, ok := relativeRoot(placed, "", "x"); ok {
		t.Error("a relative path from /p to Z:/x")
	}
	if sameRelative(placed, declared, "", "x") || !sameRelative(declared, declared, "", "x") {
		t.Error("sameRelative")
	}
}
