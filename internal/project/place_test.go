package project_test

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
)

// placeCase is a project at /p of roots a (required) and b (optional), both "../x" by default.
func placeCase(a, b string) *project.Project {
	p := project.New("acme", project.Version{Minor: 1})
	p.Roots = []project.Root{{Name: "a", Path: a}, {Name: "b", Path: b, Optional: true}}
	return p
}

// abs is where layout reads written, "" when it does not resolve.
func abs(l *project.Layout, written string) string {
	r, ok := l.Resolve(written, "", source.Span{}, diag.NewBag(nil, ""))
	if !ok {
		return ""
	}
	return r.Abs
}

// API.md O2 (DECISIONS 332): root by root, an override wins over
// project.local.canon, which wins over project.canon; a relative local path is the project's.
func TestPlacePrecedence(t *testing.T) {
	fsys := memFS{fstest.MapFS{"A/x": {}, "B/x": {}, "C/x": {}, "D/x": {}}}
	p := placeCase("../A", "../B")
	local := &project.Local{Roots: []project.Root{{Name: "a", Path: "/C"}, {Name: "b", Path: "../D"}}}
	l, ok := project.Place(p, "/p", project.Placement{Local: local, Overrides: map[string]string{"a": "/A"}, FS: fsys}, diag.NewBag(nil, ""))
	if !ok {
		t.Fatal("Place")
	}
	if got := abs(l, "@a/f"); got != "/A/f" {
		t.Errorf("@a: %s, want the override", got)
	}
	if got := abs(l, "@b/f"); got != "/D/f" {
		t.Errorf("@b: %s, want project.local.canon's", got)
	}
	if l.Moved("a") || !l.Moved("b") || l.Moved("none") {
		t.Errorf("Moved: a %v, b %v", l.Moved("a"), l.Moved("b"))
	}
}

// API.md O2 (DECISIONS 332): a root at or inside the project is present whatever the disk
// holds; an absent optional root is Absent, a present or a required one never is.
func TestPlacePresence(t *testing.T) {
	fsys := memFS{fstest.MapFS{"p/project.canon": {}}}
	for _, c := range []struct {
		name, a, b string
		ok         bool
		absentB    bool
	}{
		{"inside", "gen", ".", true, false},
		{"optional absent", "gen", "../B", true, true},
		{"required absent", "../A", "gen", false, false},
	} {
		bag := diag.NewBag(nil, "")
		l, ok := project.Place(placeCase(c.a, c.b), "/p", project.Placement{FS: fsys}, bag)
		if ok != c.ok || l.Absent("b") != c.absentB || l.Absent("a") || l.Absent("none") {
			t.Errorf("%s: ok %v, Absent(b) %v, findings %v", c.name, ok, l.Absent("b"), bag.Findings())
		}
	}
}

// DECISIONS 343: a consumer root is optional too: absent, it is Absent and never E1013.
func TestPlaceConsumerAbsent(t *testing.T) {
	fsys := memFS{fstest.MapFS{"p/project.canon": {}}}
	p := project.New("acme", project.Version{Minor: 1})
	p.Roots = []project.Root{{Name: "admin", Path: "../admin", Consumer: true}}
	bag := diag.NewBag(nil, "")
	l, ok := project.Place(p, "/p", project.Placement{FS: fsys}, bag)
	if !ok || !l.Absent("admin") || bag.ErrorCount() != 0 {
		t.Errorf("ok %v, Absent(admin) %v, findings %v", ok, l.Absent("admin"), bag.Findings())
	}
	if !p.Consumer("admin") || p.Consumer("none") {
		t.Error("admin is a consumer root, none is no root")
	}
}

// DECISIONS 332: a root whose directory is a symbolic link to a directory is present; a link to a
// file, or a dangling one, is not.
func TestPlaceSymlink(t *testing.T) {
	tmp := t.TempDir()
	proj, target := filepath.Join(tmp, "p"), filepath.Join(tmp, "target")
	for _, d := range []string{proj, target} {
		if err := os.Mkdir(d, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(tmp, "file"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for link, to := range map[string]string{"dir": target, "file": filepath.Join(tmp, "file"), "dangling": filepath.Join(tmp, "nothing")} {
		if err := os.Symlink(to, filepath.Join(tmp, "link-"+link)); err != nil {
			t.Skip("no symbolic links here:", err)
		}
	}
	for link, present := range map[string]bool{"dir": true, "file": false, "dangling": false} {
		p := placeCase("../link-"+link, "x")
		_, ok := project.Place(p, filepath.ToSlash(proj), project.Placement{FS: project.OS()}, diag.NewBag(nil, ""))
		if ok != present {
			t.Errorf("link to %s: present %v, want %v", link, ok, present)
		}
	}
}
