package ir_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/project"
)

// CODEGEN.md §2.3, §2.8, DECISIONS 229: two copies of one go emit in one directory are check's E8009 `outRoot` alone, never E8008, which compares different emits.
func TestCopiesInOneDirectoryAreNoE8008(t *testing.T) {
	w := newWorld(t)
	w.add(t, "a/a.canon", []byte("package a\n\nemit go { out: [\"@features/a/\", \"@features/x/../a/\"], package: \"a\" }\n"))
	w.build(t)
	got := reFindingCode.FindAllStringSubmatch(w.findings(t), -1)
	if len(got) != 1 || got[0][1] != string(diag.E8009.Def().Code) {
		t.Errorf("findings %v, want %s alone", got, diag.E8009.Def().Code)
	}
}

// CODEGEN.md §2.8, DECISIONS 229, 269.
func TestCopyOf(t *testing.T) {
	p := project.New("acme", project.Version{Major: 0, Minor: 1})
	p.Roots = []project.Root{{Name: "east", Path: "east"}, {Name: "west", Path: "../west"}}
	qEast := &ir.Emit{Target: ir.TargetGo, Dir: "east/q", GoImport: "example.com/east/q"}
	qWest := &ir.Emit{Target: ir.TargetGo, Dir: "../west/q", GoImport: "example.com/west/q"}
	qJSON := &ir.Emit{Target: ir.TargetJSON, Dir: "east/data"}
	only := &ir.Emit{Target: ir.TargetGo, Dir: "east/r", GoImport: "example.com/east/r"}
	pkg := &ir.Package{Name: "p", Imports: []*ir.PackageRef{
		{Name: "q", Emits: []*ir.Emit{qEast, qWest, qJSON}},
		{Name: "r", Emits: []*ir.Emit{only}},
	}}
	for _, c := range []struct {
		name  string
		e     *ir.Emit
		wantQ []*ir.Emit
	}{
		{"east copy", &ir.Emit{Target: ir.TargetGo, Dir: "east/p"}, []*ir.Emit{qEast, qJSON}},
		{"west copy", &ir.Emit{Target: ir.TargetGo, Dir: "../west/deep/p"}, []*ir.Emit{qWest, qJSON}},
		{"copy owned by the project", &ir.Emit{Target: ir.TargetGo, Dir: "out/p"}, []*ir.Emit{qEast, qWest, qJSON}},
	} {
		view := ir.CopyOf(p, pkg, c.e)
		if got := view.Imports[0].Emits; !slices.Equal(got, c.wantQ) {
			t.Errorf("%s: q's emits %v, want %v", c.name, got, c.wantQ)
		}
		if got := view.Imports[1].Emits; !slices.Equal(got, []*ir.Emit{only}) {
			t.Errorf("%s: r's only copy not kept: %v", c.name, got)
		}
		if len(pkg.Imports[0].Emits) != 3 {
			t.Fatalf("%s: CopyOf changed the package it was given", c.name)
		}
	}
	if view := ir.CopyOf(p, pkg, &ir.Emit{Target: ir.TargetCpp, Dir: "east/p"}); view != pkg {
		t.Error("a target no import has copies of narrows nothing: the package itself is returned")
	}
}

// consumerProject declares an ordinary root lib and two consumer roots, admin and web (DECISIONS 343).
const consumerProject = `project acme {
  canon: "0.1"
  roots {
    lib: "lib"
    admin: "admin"
    web: "web"
  }
  consumer_roots: [admin, web]
  go_module {
    lib: "example.com/lib"
    admin: "example.com/admin"
    web: "example.com/web"
  }
}
`

// TestConsumerCopyAllowed is DECISIONS 347's negatives: a copy may use an import's only copy under an ordinary root (a shared library root is legitimate) or under its own consumer root; only another consumer root's copy is E8004 consumerCopy (testdata/findings/E8004_6, E8004_7).
func TestConsumerCopyAllowed(t *testing.T) {
	for _, c := range []struct{ name, b, a string }{
		{"ordinary root used from a consumer root", "@lib/b/", "@admin/a/"},
		{"ordinary root used from an ordinary root", "@lib/b/", "@lib/a/"},
		{"the same consumer root", "@admin/b/", "@admin/a/"},
	} {
		w := newWorld(t)
		w.useProject(t, []byte(consumerProject))
		w.add(t, "b/b.canon", []byte("package b\n\n/// A colour.\nenum Tone { red, blue }\n\nemit go { out: \""+c.b+"\" }\n"))
		w.add(t, "a/a.canon", []byte("package a\n\nimport b { Tone }\n\n/// A badge.\nrecord Badge {\n  /// Its colour.\n  tone: Tone\n}\n\nemit go { out: \""+c.a+"\" }\n"))
		w.build(t)
		if out := w.findings(t); !strings.HasPrefix(out, noFindings) {
			t.Errorf("%s: want no finding:\n%s", c.name, out)
		}
	}
}

// TestCopyOfConsumer is DECISIONS 347: a copy sees no copy of an import whose only copy lies under another consumer root (so a decoded @text result reaching it has no decoder), and keeps one under an ordinary root or its own consumer root.
func TestCopyOfConsumer(t *testing.T) {
	p := project.New("acme", project.Version{Major: 0, Minor: 1})
	p.Roots = []project.Root{{Name: "lib", Path: "lib"}, {Name: "admin", Path: "admin", Consumer: true}, {Name: "web", Path: "web", Consumer: true}}
	qAdmin := &ir.Emit{Target: ir.TargetGo, Dir: "admin/q"}
	rLib := &ir.Emit{Target: ir.TargetGo, Dir: "lib/r"}
	pkg := &ir.Package{Name: "p", Imports: []*ir.PackageRef{{Name: "q", Emits: []*ir.Emit{qAdmin}}, {Name: "r", Emits: []*ir.Emit{rLib}}}}
	for _, c := range []struct {
		name  string
		dir   string
		wantQ int
	}{
		{"another consumer root", "web/p", 0},
		{"an ordinary root", "lib/p", 0},
		{"the same consumer root", "admin/deep/p", 1},
	} {
		view := ir.CopyOf(p, pkg, &ir.Emit{Target: ir.TargetGo, Dir: c.dir})
		if got := len(view.Imports[0].Emits); got != c.wantQ {
			t.Errorf("%s: q's copies %d, want %d", c.name, got, c.wantQ)
		}
		if got := view.Imports[1].Emits; !slices.Equal(got, []*ir.Emit{rLib}) {
			t.Errorf("%s: r's only copy, under an ordinary root, not kept: %v", c.name, got)
		}
	}
	if len(pkg.Imports[0].Emits) != 1 {
		t.Fatal("CopyOf changed the package it was given")
	}
}
