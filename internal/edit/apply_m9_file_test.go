package edit_test

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/project"
)

// aliasProject declares two roots over one directory; narrow, when given, reads x.json through
// the second as a Float32 while wide reads it through the first as a Float.
func aliasProject(narrow bool) mapFS {
	src := "/// P.\npackage d\n\n/// Wide.\nlet wide: Float = load(\"@a/x.json\", at: \"x\")\n"
	if narrow {
		src += "\n/// Narrow.\nlet narrow: Float32 = load(\"@b/x.json\", at: \"x\")\n"
	}
	return mapFS{
		"law/project.canon": file("project acme {\n  canon: \"0.1\"\n\n  roots {\n    a: \"data\"\n    b: \"data\"\n  }\n}\n"),
		"law/d/d.canon":     file(src),
		"law/data/x.json":   file(aliasJSON),
	}
}

const aliasJSON = "{\n  \"x\": 1.0\n}\n"

// API.md M9, FORMATTER.md 14.1 (log-2026-09-29 M4 B11-r5): typed numbers are judged per file, so
// the readings through two roots over one directory are one file's: read as two types, the token
// is kept, under either display path; read through one root as one type, it is canonical.
func TestM9AliasedRoots(t *testing.T) {
	for _, c := range []struct {
		name, display, want string
		narrow              bool
	}{
		{"two types, first root", "@a/x.json", aliasJSON, true},
		{"two types, second root", "@b/x.json", aliasJSON, true},
		{"one type", "@a/x.json", "{\n  \"x\": 1\n}\n", false},
	} {
		s := open(t, aliasProject(c.narrow), nil, "", "d")
		out, err := edit.CanonicalJSON(s.snap, c.display, []byte(aliasJSON))
		if err != nil || string(out) != c.want {
			t.Errorf("%s: %q, %v; want %q", c.name, out, err, c.want)
		}
	}
}

// API.md M9 (log-2026-09-29 M4 B11-r5): a link and its target are one file: the readings through
// both are judged together.
func TestM9Symlink(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"project.canon": projectCanon, "data/x.json": aliasJSON,
		"d/d.canon": "/// P.\npackage d\n\n/// Wide.\nlet wide: Float = load(\"../data/x.json\", at: \"x\")\n\n/// Narrow.\nlet narrow: Float32 = load(\"../data/y.json\", at: \"x\")\n",
	}
	for name, text := range files { //canon:unordered each file is written alone
		p := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), dirPerm); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(text), filePerm); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink("x.json", filepath.Join(dir, "data", "y.json")); err != nil {
		t.Skipf("no symbolic links here: %v", err)
	}
	p, err := build.Open(project.OS(), dir, build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	a, err := p.Analyze(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, display := range []string{"data/x.json", "data/y.json"} {
		out, err := edit.CanonicalJSON(edit.NewSnapshot(a), display, []byte(aliasJSON))
		if err != nil || string(out) != aliasJSON {
			t.Errorf("%s: %q, %v; want the token kept", display, out, err)
		}
	}
}

// flipFS serves name's bytes, then other on every later read of it: a file changing between two loads.
type flipFS struct {
	mapFS
	name  string
	other []byte
	mu    sync.Mutex
	reads int
}

func (f *flipFS) ReadFile(name string) ([]byte, error) {
	if rel(name) != f.name {
		return f.mapFS.ReadFile(name)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.reads++; f.reads > 1 {
		return f.other, nil
	}
	return f.mapFS.ReadFile(name)
}

// API.md M9 (log-2026-09-29 M4 B11-r5): two loads that read different contents of one file give
// no typed reading: its numbers are left as written.
func TestM9MixedContents(t *testing.T) {
	fsys := &flipFS{mapFS: aliasProject(false), name: "law/data/x.json", other: []byte("{\n  \"x\": 2.0\n}\n")}
	fsys.mapFS["law/d/d.canon"] = file("/// P.\npackage d\n\n/// Wide.\nlet wide: Float = load(\"@a/x.json\", at: \"x\")\n\n/// Again.\nlet again: Float = load(\"@b/x.json\", at: \"x\")\n")
	p, err := build.Open(fsys, "/law", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	a, err := p.Analyze(context.Background(), []string{"d"})
	if err != nil {
		t.Fatal(err)
	}
	if fsys.reads < 2 {
		t.Fatalf("x.json read %d times, want two contents", fsys.reads)
	}
	out, err := edit.CanonicalJSON(edit.NewSnapshot(a), "@a/x.json", []byte(aliasJSON))
	if err != nil || string(out) != aliasJSON {
		t.Errorf("%q, %v; want the token kept", out, err)
	}
}
