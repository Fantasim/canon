package canon

import (
	"context"
	"errors"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// The packages of the walk-base tests: e's items one directory each under items/, f's one flat
// directory, g walking e's axe directory, h e's bow directory through its link h/bl, l's layer
// dev e's axe directory, d's entry wolf its own directory, k a flat directory beside a let n.
const (
	walkE = "/// E.\npackage e\n\n/// Item.\nrecord Item {\n  /// V.\n  v: Int = 1\n}\n\n/// Items.\n" +
		"@files(\"items/{id}/{id}.json\")\nlet items: table Item = load.dir(\"items/*/*.json\")\n"
	walkF = "/// F.\npackage f\n\n/// T.\nrecord T {\n  /// V.\n  v: Int = 1\n}\n\n/// Ts.\nlet ts: table T = load.dir(\"ts/*.json\")\n"
	walkG = "/// G.\npackage g\n\n/// G.\nrecord G {\n  /// V.\n  v: Int = 1\n}\n\n/// Gs.\nlet gs: [G] = load.dir(\"../e/items/axe/*.json\")\n"
	walkH = "/// H.\npackage h\n\n/// H.\nrecord H {\n  /// V.\n  v: Int = 1\n}\n\n/// Hs.\nlet hs: [H] = load.dir(\"bl/*.json\")\n"
	walkL = "/// L.\npackage l\n\n/// G.\nrecord G {\n  /// V.\n  v: Int = 1\n}\n\n/// Box.\nrecord Box {\n  /// Gs.\n" +
		"  gs: [G] = []\n}\n\n/// The box.\nlet box: Box = {}\n"
	walkLDev = "package l\nlayer dev\n\namend box {\n  gs: load.dir(\"../e/items/axe/*.json\")\n}\n"
	walkD    = "/// D.\npackage d\n\n/// M.\nrecord M {\n  /// Hp.\n  hp: Int = 1\n  /// Drops.\n  drops: [Int] = []\n}\n\n" +
		"/// Ms.\n@files(\"m/{id}/{id}.canon\")\nlet ms: table M = {}\n"
	walkK = "/// K.\npackage k\n\n/// T.\nrecord T {\n  /// V.\n  v: Int = 1\n}\n\n/// Ts.\nlet ts: table T = load.dir(\"ts/*.json\")\n\n" +
		"/// N.\nlet n: Int = 1\n"

	walkEntry = "{\n  \"v\": 2\n}\n"
	walkItems = "e:items"
)

// walkProject writes the project of the packages named, h's link h/bl included, and opens it with opt.
func walkProject(t *testing.T, opt Options, pkgs ...string) (*Project, string) {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{"project.canon": "project acme {\n  canon: \"0.1\"\n}\n"}
	sources := map[string]map[string]string{
		"e": {"e/e.canon": walkE, "e/items/axe/axe.json": walkEntry, "e/items/bow/bow.json": walkEntry},
		"f": {"f/f.canon": walkF, "f/ts/one.json": walkEntry},
		"g": {"g/g.canon": walkG},
		"h": {"h/h.canon": walkH},
		"l": {"l/l.canon": walkL, "l/dev.layer.canon": walkLDev},
		"d": {"d/d.canon": walkD, "d/m/wolf/wolf.canon": "package d\n\nentry ms.wolf { drops: load.dir(\"./*.json\") }\n",
			"d/m/bat/bat.canon": "package d\n\nentry ms.bat { hp: 2 }\n"},
		"k": {"k/k.canon": walkK, "k/ts/one.json": walkEntry},
	}
	for _, pkg := range pkgs {
		for name, text := range sources[pkg] { //canon:unordered each file is written alone
			files[name] = text
		}
	}
	linksWrite(t, dir, files)
	if slices.Contains(pkgs, "h") {
		if err := os.Symlink("../e/items/bow", filepath.Join(dir, "h", "bl")); err != nil {
			t.Skipf("no symbolic links here: %v", err)
		}
	}
	p, err := Open(dir, opt)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	return p, dir
}

// walkEdit applies ops, which must be accepted without AllowErrors and add no error to canon check.
func walkEdit(t *testing.T, p *Project, ops ...Op) *EditResult {
	t.Helper()
	before := checkErrors(t, p)
	res, err := p.Edit(context.Background(), Edit{Ops: ops})
	if err != nil {
		t.Fatalf("API.md E19: %v: %v %+v", ops, err, res)
	}
	if after := checkErrors(t, p); !slices.Equal(before, after) {
		t.Errorf("API.md E18: %v took canon check's errors from %v to %v", ops, before, after)
	}
	return res
}

// walkDirs fails unless each name below dir is a directory holding exactly want entries, -1 for
// a name that must not exist.
func walkDirs(t *testing.T, dir string, want map[string]int) {
	t.Helper()
	for _, name := range slices.Sorted(maps.Keys(want)) {
		list, err := os.ReadDir(filepath.Join(dir, filepath.FromSlash(name)))
		switch n := want[name]; {
		case n < 0 && !errors.Is(err, fs.ErrNotExist):
			t.Errorf("API.md N6: %s is %v, %v; want it removed", name, list, err)
		case n >= 0 && (err != nil || len(list) != n):
			t.Errorf("API.md N6, WIRE.md 6.5: %s is %v, %v; want %d entries", name, list, err, n)
		}
	}
}

// API.md N6 (DECISIONS 482), E22; WIRE.md 6.5 (LOD-05): removing every entry of a nested load.dir
// removes each entry's directory but keeps the walk's base, so the table is empty, not E7004; its
// Undo brings both files back, and an AddEntry into the emptied table creates its file (N1-N3).
func TestEditEmptiesNestedWalk(t *testing.T) {
	p, dir := walkProject(t, Options{}, "e")
	items := valueText(t, p, walkItems)
	res := walkEdit(t, p, Remove("e:items.axe"), Remove("e:items.bow"))
	walkDirs(t, dir, map[string]int{"e/items": 0, "e/items/axe": -1, "e/items/bow": -1})
	if got := valueText(t, p, walkItems); got == items {
		t.Errorf("API.md N6: e:items is still %s", got)
	}
	if _, err := p.Edit(context.Background(), Edit{Base: res.Revision, Ops: res.Undo}); err != nil {
		t.Fatalf("API.md E22: the Undo %v: %v", res.Undo, err)
	}
	if got := valueText(t, p, walkItems); got != items {
		t.Errorf("API.md E22: e:items after the Undo is %s, want %s", got, items)
	}
	walkDirs(t, dir, map[string]int{"e/items/axe": 1, "e/items/bow": 1})
	walkEdit(t, p, Remove("e:items.axe"), Remove("e:items.bow"))
	walkEdit(t, p, AddEntry(walkItems, Key("cut"), Source("{ v: 3 }")))
	walkDirs(t, dir, map[string]int{"e/items": 1, "e/items/cut": 1})
}

// API.md N6 (DECISIONS 482), N4; WIRE.md 6.5 (LOD-05): removing the last entry of a flat load.dir
// keeps its directory, and an AddEntry into the emptied table goes there, as <key>.json.
func TestEditEmptiesFlatWalk(t *testing.T) {
	p, dir := walkProject(t, Options{}, "f")
	walkEdit(t, p, Remove("f:ts.one"))
	walkDirs(t, dir, map[string]int{"f/ts": 0})
	walkEdit(t, p, AddEntry("f:ts", Key("two"), Source("{ v: 2 }")))
	if _, err := os.Stat(filepath.Join(dir, "f", "ts", "two.json")); err != nil {
		t.Errorf("API.md N4: the new entry's file: %v", err)
	}
}

// API.md N6 (DECISIONS 482), E17; WIRE.md 6.5 (LOD-05): an entry directory another package's walk
// starts at, by its own path (g) or through a link (h), is kept when emptied; the other one goes.
func TestEditKeepsOtherWalkBase(t *testing.T) {
	for _, c := range []struct{ pkg, kept, gone string }{{"g", "e/items/axe", "e/items/bow"}, {"h", "e/items/bow", "e/items/axe"}} {
		t.Run(c.pkg, func(t *testing.T) {
			p, dir := walkProject(t, Options{}, "e", c.pkg)
			walkEdit(t, p, Remove("e:items.axe"), Remove("e:items.bow"))
			walkDirs(t, dir, map[string]int{"e/items": 1, c.kept: 0, c.gone: -1})
		})
	}
}

// API.md N6 (DECISIONS 482), W11: a layer's load.dir names its walk's base as a source's does,
// whether the layer is active or not: the emptied directory stays.
func TestEditKeepsLayerWalkBase(t *testing.T) {
	for _, layers := range [][]string{nil, {"dev"}} {
		p, dir := walkProject(t, Options{Layers: layers}, "e", "l")
		walkEdit(t, p, Remove("e:items.axe"), Remove("e:items.bow"))
		walkDirs(t, dir, map[string]int{"e/items": 1, "e/items/axe": 0, "e/items/bow": -1})
	}
}

// API.md N6 (DECISIONS 482): only the walks of the sources the edit leaves count. A directory only
// the deleted file's load.dir named goes with it; a file the edit rewrites keeps its walk's base.
func TestEditWalkBaseOfFinalSources(t *testing.T) {
	p, dir := walkProject(t, Options{}, "d")
	walkEdit(t, p, Remove("d:ms.wolf"))
	walkDirs(t, dir, map[string]int{"d/m": 1, "d/m/wolf": -1})
	p, dir = walkProject(t, Options{}, "k")
	walkEdit(t, p, Remove("k:ts.one"), Set("k:n", Int(2)))
	walkDirs(t, dir, map[string]int{"k/ts": 0})
}
