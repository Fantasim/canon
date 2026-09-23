package project_test

import (
	"context"
	"errors"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"golang.org/x/tools/txtar"
)

const tree = `
-- project.canon --
project acme {
  canon: "0.1"
}
-- .git/x.canon --
package hidden
-- game/items/item.canon --
package game.items

import shared.ui
import resource.vocab as vocab
-- game/items/entries/IK1/axe.canon --
package game.items

import resource.vocab
-- game/items/item.fr.canon --
package game.items
translation fr
-- game/items/knights.layer.canon --
package game.items
layer knights
-- game/broken/broken.canon --
const X = 1
-- shared/ui/ui.canon --
package shared.ui

const = 1
-- tools/project.canon --
package tools
-- lib/deep/x.canon --
package lib
-- mixed/a.canon --
package mixed
-- mixed/b.canon --
package shared.ui
-- readme.md --
not a source
`

func parseTree(t *testing.T) ([]*project.Unit, map[string]*diag.Bag, *project.Reader) {
	t.Helper()
	fsys := newMemFS(txtar.Parse([]byte(tree)))
	names, err := project.Scan(fsys, projectDir)
	if err != nil {
		t.Fatal(err)
	}
	set := &source.FileSet{}
	bags := map[string]*diag.Bag{}
	bagOf := func(pkg string) *diag.Bag {
		if bags[pkg] == nil {
			bags[pkg] = diag.NewBag(set, pkg)
		}
		return bags[pkg]
	}
	r := &project.Reader{FS: fsys, Dir: projectDir, Set: set, BagOf: bagOf}
	units, err := r.Parse(context.Background(), names)
	if err != nil {
		t.Fatal(err)
	}
	return units, bags, r
}

// API.md O2: the scan skips "." directories and the top project.canon, in byte order.
func TestScan(t *testing.T) {
	fsys := newMemFS(txtar.Parse([]byte(tree)))
	got, err := project.Scan(fsys, projectDir)
	want := []string{"game/broken/broken.canon", "game/items/entries/IK1/axe.canon", "game/items/item.canon",
		"game/items/item.fr.canon", "game/items/knights.layer.canon", "lib/deep/x.canon", "mixed/a.canon", "mixed/b.canon",
		"shared/ui/ui.canon", "tools/project.canon"}
	if err != nil || !slices.Equal(got, want) {
		t.Errorf("Scan = %q, %v", got, err)
	}
	if _, err := project.Scan(fsys, "/missing"); err == nil {
		t.Error("Scan of a missing directory succeeded")
	}
}

// SPEC §3.2: a file joins the package its package line names; one without joins none.
func TestReaderGroupsByPackage(t *testing.T) {
	units, bags, r := parseTree(t)
	var got []string
	for _, u := range units {
		got = append(got, u.Name+" "+u.Dir+" "+strings.Join(u.Imports, ",")+" "+strings.Join(u.Layers, ",")+" "+
			strings.Repeat("f", len(u.Files)))
	}
	want := []string{"game.items game/items resource.vocab,shared.ui knights ffff", "lib lib   f", "mixed mixed   f",
		"shared.ui shared/ui   ff", "tools tools   f"}
	if !slices.Equal(got, want) {
		t.Errorf("units:\n%s", strings.Join(got, "\n"))
	}
	for pkg, n := range map[string]int{"": 1, "game.broken": 0, "shared.ui": 1, "game.items": 0} {
		if b := bags[pkg]; (b == nil && n > 0) || (b != nil && len(b.Findings()) != n) {
			t.Errorf("bag %s: want %d findings", pkg, n)
		}
	}
	if len(r.Sums) != 10 {
		t.Errorf("%d files read", len(r.Sums))
	}
}

// CLI.md §2.2, API.md R1: each selector form; an unknown one is ErrUnknownPackage.
func TestSelect(t *testing.T) {
	units, _, _ := parseTree(t)
	for _, c := range []struct {
		sel  []string
		want string
	}{
		{nil, "game.items lib mixed shared.ui tools"},
		{[]string{"./lib", "./lib/deep"}, "lib"},
		{[]string{"game..."}, "game.items"},
		{[]string{"./game/items/entries/IK1", "./shared/ui/"}, "game.items shared.ui"},
		{[]string{"game.items", "game.items..."}, "game.items"},
		{[]string{"./game/items", "shared/ui/ui.canon"}, "game.items shared.ui"},
		{[]string{"game/items/entries/IK1/axe.canon"}, "game.items"},
	} {
		got, err := project.Select(units, c.sel)
		var names []string
		for _, u := range got {
			names = append(names, u.Name)
		}
		if err != nil || strings.Join(names, " ") != c.want {
			t.Errorf("Select(%q) = %q, %v", c.sel, names, err)
		}
	}
	// ./game/items/entries holds files only in a subdirectory and names no package: unknown.
	for _, sel := range []string{"game", "game/items", "./game/items/entries", "gam...", "x.canon", "./game/broken"} {
		if _, err := project.Select(units, []string{sel}); !errors.Is(err, project.ErrUnknownPackage) {
			t.Errorf("Select(%q) = %v", sel, err)
		}
	}
	var ue *project.UnknownError
	if _, err := project.Select(units, []string{"./mixed"}); !errors.Is(err, project.ErrMixedDirectory) ||
		!errors.As(err, &ue) || ue.Name != "./mixed" {
		t.Errorf("Select(./mixed) = %v", err)
	}
}

// GRAMMAR.md §2.3, §9.2, DECISIONS 137: package names for `canon new`.
func TestIsPackageName(t *testing.T) {
	for name, want := range map[string]bool{"game.items": true, "a": true, "sovcommon.ui2": true, "Game": false,
		"game.match": false, "_x": false, "a..b": false, "": false, "a-b": false, "ik1_weapon": false} {
		if got := project.IsPackageName(name); got != want {
			t.Errorf("IsPackageName(%q) = %v", name, got)
		}
	}
	if project.IsIdent("type") || !project.IsIdent("IK1_WEAPON") || project.IsIdent("_") {
		t.Error("IsIdent")
	}
}

// GRAMMAR.md §7.1: examples/project.canon loads clean; errors say which kind stopped a load.
func TestLoad(t *testing.T) {
	data, err := os.ReadFile("../../examples/project.canon")
	if err != nil {
		t.Fatal(err)
	}
	set := &source.FileSet{}
	src, _ := set.Add(project.FileName, "/examples/project.canon", data)
	bag := diag.NewBag(set, "")
	p, err := project.Load(src, bag)
	if err != nil || len(bag.Findings()) != 0 {
		t.Fatalf("Load: %v, %d findings", err, len(bag.Findings()))
	}
	if p.Name != "sovereign" || len(p.Roots) != 10 || p.Studio.Path != "studio" || p.Budget != 100_000_000 ||
		!slices.Equal(p.Languages, []string{"en", "fr"}) || len(p.GoModules) != 4 {
		t.Errorf("project %+v", p)
	}
	for text, want := range map[string]error{
		"project a {\n  canon: \"0.9\"\n}\n": project.ErrUnsupportedVersion,
		"project a {\n}\n":                   project.ErrInvalid,
		"package a\n":                        project.ErrInvalid,
	} {
		src, _ := set.Add(project.FileName, "/x/project.canon", []byte(text))
		if p, err := project.Load(src, diag.NewBag(set, "")); p != nil || !errors.Is(err, want) {
			t.Errorf("Load(%q) = %v, %v", text, p, err)
		}
	}
}

// CLI.md §2.1, IMPLEMENTATION-PLAN.md §10: the nearest project.canon, its name matched exactly.
func TestFind(t *testing.T) {
	fsys := newMemFS(txtar.Parse([]byte(tree)))
	fsys.m["p/mixed/Project.canon"] = fsys.m["p/project.canon"]
	for dir, want := range map[string]string{"/p/game/items/entries": "/p", "/p": "/p", "/p/tools": "/p/tools",
		"/p/mixed": "/p", "/p/nowhere/x": "/p"} {
		if got, err := project.Find(fsys, dir, nil); err != nil || got != want {
			t.Errorf("Find(%s) = %s, %v", dir, got, err)
		}
	}
	set := &source.FileSet{}
	bag := diag.NewBag(set, "")
	if err := project.Require(fsys, "/p/mixed", bag); !errors.Is(err, project.ErrNoProject) || len(bag.Findings()) != 1 {
		t.Errorf("Require(/p/mixed) = %v", err)
	}
}
