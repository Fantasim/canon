package ir_test

import (
	"slices"
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
