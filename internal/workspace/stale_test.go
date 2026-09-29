package workspace_test

import (
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/workspace"
)

// staleFiles is the files a Stale error names, and whether it is one.
func staleFiles(err error) ([]string, bool) {
	var se *workspace.StaleError
	if !errors.As(err, &se) {
		return nil, false
	}
	return se.Files, errors.Is(err, workspace.ErrStale)
}

// API.md S5, S6: staleness is judged on the read set of the packages touched: a change to an
// unrelated file is not stale, one to a package's own file, its import's or a file its load read
// is; Base "" is never stale.
func TestStalePerPackage(t *testing.T) {
	fsys := newMemFS(lawFiles())
	p := open(t, fsys)
	s := read(t, p)
	a := analyze(t, s)
	base := revision(t, s)
	_ = fsys.WriteFile("/law/data/c.json", []byte("[9]\n"))
	_ = fsys.WriteFile("/law/a/a.json", []byte("[7]\n"))
	now := read(t, p)
	for _, c := range []struct {
		pkg   string
		files []string
	}{
		{"a", []string{"a/a.json"}},
		{"b", []string{"a/a.json"}},
		{"c", []string{"@data/c.json"}},
	} {
		if files, ok := staleFiles(now.Stale(base, a.Reads(c.pkg))); !ok || !slices.Equal(files, c.files) {
			t.Errorf("Stale(%s) names %v, want %v", c.pkg, files, c.files)
		}
	}
	if err := now.Stale("", a.Reads("a")); err != nil {
		t.Errorf("Base \"\" is stale: %v (S6)", err)
	}
	if err := now.Stale(revision(t, now), a.Reads("a")); err != nil {
		t.Errorf("the current revision is stale: %v", err)
	}
	_ = fsys.WriteFile("/law/b/b.canon", []byte(srcB+"\n"))
	if err := read(t, p).Stale(revision(t, now), a.Reads("c")); err != nil {
		t.Errorf("an unrelated change made c stale: %v", err)
	}
}

// API.md S4: the last 64 revisions produced are remembered; an older one, or one this project
// never produced, is stale with no file named.
func TestStaleHistory(t *testing.T) {
	fsys := newMemFS(lawFiles())
	p := open(t, fsys)
	a := analyze(t, read(t, p))
	var revs []string
	for i := range 70 {
		_ = fsys.WriteFile("/law/a/a.json", []byte(fmt.Sprintf("[%d]\n", i)))
		revs = append(revs, revision(t, read(t, p)))
	}
	now := read(t, p)
	reads := a.Reads("b")
	for i, rev := range revs {
		files, ok := staleFiles(now.Stale(rev, reads))
		switch {
		case i < len(revs)-64:
			if !ok || files != nil {
				t.Errorf("revision %d of 70: %v, %v, want forgotten", i, files, ok)
			}
		case i < len(revs)-1:
			if !ok || !slices.Equal(files, []string{"a/a.json"}) {
				t.Errorf("revision %d of 70: %v, %v, want a/a.json changed", i, files, ok)
			}
		default:
			if ok {
				t.Errorf("the current revision is stale: %v", files)
			}
		}
	}
	other := open(t, newMemFS(map[string]string{"/law/project.canon": lawProject, "/law/z/z.canon": "/// Z.\npackage z\n"}))
	if _, ok := staleFiles(now.Stale(revision(t, read(t, other)), reads)); !ok {
		t.Error("another project's revision is not stale")
	}
}

// API.md S5, O2: a name added to a touched package's directory makes it stale exactly when the
// scan reads it as a source, dot-prefixed ones included, and the error names it; a swap file or
// any other name the scan skips does not; a source added to a does not make c stale.
func TestStaleAddedFile(t *testing.T) {
	for _, c := range []struct {
		name  string
		stale bool
	}{
		{".a.canon.swp", false}, {"#a.canon#", false}, {"notes.txt", false},
		{".hidden.canon", true}, {".#a.canon", true}, {"more.canon", true},
	} {
		fsys := newMemFS(lawFiles())
		p := open(t, fsys)
		s := read(t, p)
		a := analyze(t, s)
		base := revision(t, s)
		_ = fsys.WriteFile("/law/a/"+c.name, []byte("/// More.\npackage a\n"))
		now := read(t, p)
		files, stale := staleFiles(now.Stale(base, a.Reads("b")))
		if stale != c.stale || (c.stale && !slices.Equal(files, []string{"a/" + c.name})) {
			t.Errorf("%s: Stale(b) names %v (stale %v), want stale %v", c.name, files, stale, c.stale)
		}
		if err := now.Stale(base, a.Reads("c")); err != nil {
			t.Errorf("%s added to a made c stale: %v", c.name, err)
		}
	}
}

// API.md S5: a source removed from a touched package's directory makes it stale, naming the
// directory, whose sources no longer match.
func TestStaleRemovedFile(t *testing.T) {
	files := lawFiles()
	files["/law/a/two.canon"] = "/// Two.\npackage a\n"
	fsys := newMemFS(files)
	p := open(t, fsys)
	s := read(t, p)
	a := analyze(t, s)
	base := revision(t, s)
	_ = fsys.Remove("/law/a/two.canon")
	if names, ok := staleFiles(read(t, p).Stale(base, a.Reads("a"))); !ok || !slices.Contains(names, "a") {
		t.Errorf("Stale(a) names %v, want the directory a", names)
	}
}

// API.md S5: a load.dir through a linked directory; retargeting the link makes the package
// stale, though no file it read changed.
func TestStaleRetargetedLink(t *testing.T) {
	fsys := newMemFS(map[string]string{
		"/law/project.canon": lawProject,
		"/law/l/l.canon": "/// L.\npackage l\n\n/// An item.\nrecord Item {\n  /// Id.\n  id: String\n}\n\n" +
			"/// Items.\nlet items: [Item] keyed by id = load.dir(\"@data/items/*.json\")\n",
		"/law/data/v1/a.json": "{\"id\": \"a\"}\n", "/law/data/v2/b.json": "{\"id\": \"b\"}\n",
	})
	fsys.link("/law/data/items", "/law/data/v1")
	p := open(t, fsys)
	s := read(t, p)
	a := analyze(t, s)
	if a.Result().Summary.Errors != 0 {
		t.Fatalf("findings: %v", a.Result().List)
	}
	base := revision(t, s)
	fsys.link("/law/data/items", "/law/data/v2")
	if files, ok := staleFiles(read(t, p).Stale(base, a.Reads("l"))); !ok || !slices.Contains(files, "data/items") {
		t.Errorf("Stale(l) names %v, want the retargeted data/items", files)
	}
}

// API.md S5: two packages load one header, which the loader reads once; a change to it makes
// either package stale.
func TestStaleSharedHeader(t *testing.T) {
	src := func(pkg, name string) string {
		return "/// P.\npackage " + pkg + "\n\nlocal let m = load.defines(\"@data/h.h\")\n\n/// V.\nlet v: Int = m." + name + ".value\n"
	}
	fsys := newMemFS(map[string]string{
		"/law/project.canon": lawProject, "/law/data/h.h": "#define A 1\n#define B 2\n",
		"/law/d1/d1.canon": src("d1", "A"), "/law/d2/d2.canon": src("d2", "B"),
	})
	p := open(t, fsys)
	s := read(t, p)
	a := analyze(t, s)
	if a.Result().Summary.Errors != 0 {
		t.Fatalf("findings: %v", a.Result().List)
	}
	base := revision(t, s)
	_ = fsys.WriteFile("/law/data/h.h", []byte("#define A 1\n#define B 3\n"))
	now := read(t, p)
	for _, pkg := range []string{"d1", "d2"} {
		if files, ok := staleFiles(now.Stale(base, a.Reads(pkg))); !ok || !slices.Equal(files, []string{"@data/h.h"}) {
			t.Errorf("Stale(%s) names %v, want @data/h.h", pkg, files)
		}
	}
}
