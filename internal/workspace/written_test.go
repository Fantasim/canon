package workspace_test

import (
	"context"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/workspace"
)

// filedItems is a package whose entries each live in a file its @files template names, items by
// kind and kept each in a directory of its own, and one that loads the journal's directory.
var filedItems = map[string]string{
	"project.canon": "project acme {\n  canon: \"0.1\"\n}\n",
	"game/items/item.canon": "package game.items\n\n/// What an item is.\nvariant Kind {\n  /// A material.\n  general @json(\"IK1_GENERAL\")\n" +
		"  /// A weapon.\n  weapon @json(\"IK1_WEAPON\")\n}\n\n/// An item.\nrecord Item {\n  /// Its level.\n  level: Int = 1\n" +
		"  /// What it is.\n  kind: Kind\n}\n\n/// Items by kind.\n@files(\"entries/{kind}/{id}.canon\")\nlet items: table Item = {}\n\n" +
		"/// Items, each in a directory of its own.\n@files(\"byid/{id}/item.canon\")\nlet kept: table Item = {}\n",
	"game/items/entries/IK1_GENERAL/II_MOON.canon": "package game.items\n\n/// Moonstone.\nentry items.II_MOON { level: 4, kind: general }\n",
	"game/items/entries/IK1_WEAPON/II_AXE.canon":   "package game.items\n\n/// An axe.\nentry items.II_AXE { level: 30, kind: weapon }\n",
	"game/items/byid/K1/item.canon":                "package game.items\n\n/// K1.\nentry kept.K1 { level: 2, kind: general }\n",
	"j/j.canon":                                    "/// J.\npackage j\n\n/// The journal, as a package may load it.\nlet js: [Int] = load.dir(\"../.canon/journal/*.json\")\n",
}

// onDisk is files written in a temporary directory, opened on the OS's file system and read,
// analyzed and revised, as a client leaves it before an edit.
func onDisk(t *testing.T, files map[string]string) (string, *workspace.Project) {
	t.Helper()
	dir := t.TempDir()
	for name, text := range files {
		writeOS(t, filepath.Join(dir, filepath.FromSlash(name)), text)
	}
	return filepath.ToSlash(dir), openOS(t, filepath.ToSlash(dir))
}

// openOS is a fresh project over dir on the OS's file system, read, analyzed and revised.
func openOS(t *testing.T, dir string) *workspace.Project {
	t.Helper()
	b, err := build.Open(build.OS(), dir, build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	p := workspace.New(b)
	t.Cleanup(p.Close)
	s := read(t, p)
	analyze(t, s)
	revision(t, s)
	return p
}

// sameAsFresh fails the test unless s, published by an edit, gives the revision and the build
// inputs a fresh project gives of the disk the edit left (log-2026-09-29 M4 PA3-r).
func sameAsFresh(t *testing.T, step string, s, fresh *workspace.Snapshot, names []string) {
	t.Helper()
	got, want := revision(t, s), revision(t, fresh)
	inputs, _ := s.Build().Inputs()
	freshInputs, _ := fresh.Build().Inputs()
	if got != want || !slices.Equal(inputs, freshInputs) || !slices.Equal(workspace.ListedInputs(s), freshInputs) {
		t.Errorf("%s: revision %s inputs %v, a fresh read %s inputs %v", step, got, inputs, want, freshInputs)
	}
	if _, err := s.Build().Analyze(context.Background(), nil); err != nil {
		t.Errorf("%s: the published snapshot does not analyze: %v", step, err)
	}
	for _, rel := range names {
		if got, want := seen(s, rel), seen(fresh, rel); got != want {
			t.Errorf("%s: the published snapshot sees %s as %s, a fresh read as %s", step, rel, got, want)
		}
	}
}

// editedNames are the names the edits of TestEditUnwatchedSnapshotIsTheDisk change, the ones
// above them, and the journal's directory (API.md N10).
var editedNames = []string{"", "game", "game/items", "game/items/entries", "game/items/entries/IK1_GENERAL",
	"game/items/byid", "game/items/byid/K1", "game/items/byid/K2", "game/items/byid/K2/item.canon",
	".canon", ".canon/journal"}

// seen is what s reads at rel below its project: whether it is a directory, where its links
// lead, and its names.
func seen(s *workspace.Snapshot, rel string) string {
	abs := path.Join(s.Build().Dir(), rel)
	info, err := s.Build().FS().Stat(abs)
	list, lerr := s.Build().FS().ReadDir(abs)
	real, rerr := project.EvalSymlinks(s.Build().FS(), abs)
	names := []string{fmt.Sprint(err == nil && info.IsDir(), lerr == nil, rerr == nil, real == abs)}
	for _, e := range list {
		names = append(names, e.Name())
	}
	return strings.Join(names, " ")
}

// API.md S1, S10, N6, N10 (log-2026-09-29 M4 PA3-r): with no watch, an edit that empties a
// directory, by a remove or a rename across directories, or creates the journal's, publishes what
// a fresh read of the disk gives.
func TestEditUnwatchedSnapshotIsTheDisk(t *testing.T) {
	dir, p := onDisk(t, filedItems)
	for _, c := range []struct {
		step string
		op   edit.Operation
	}{
		{"remove", edit.Operation{Kind: edit.OpRemove, Path: "game.items:items.II_MOON"}},
		{"rename", edit.Operation{Kind: edit.OpRename, Path: "game.items:kept.K1", Key: edit.Key("K2")}},
	} {
		out := commit(t, p, workspace.Changes{Host: build.EditHost, Ops: []edit.Operation{c.op}})
		sameAsFresh(t, c.step, out.After, read(t, openOS(t, dir)), editedNames)
	}
	if _, err := os.Stat(filepath.FromSlash(dir + "/game/items/byid/K1")); !os.IsNotExist(err) {
		t.Errorf("API.md N6: the directory the rename emptied is left: %v", err)
	}
}
