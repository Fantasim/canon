package canon

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

// createdItemsA is a whose items are JSON files under items/<kind>/, which load.dir reads back.
const createdItemsA = "/// A.\npackage a\n\n/// Kind.\nenum Kind { weapon, potion }\n\n/// Item.\nrecord Item {\n  /// K.\n" +
	"  kind: Kind\n  /// V.\n  v: Int(..5) = 1\n}\n\n/// Items.\n@files(\"items/{kind}/{id}.json\")\n" +
	"let items: table Item = load.dir(\"items/**/*.json\")\n"

// createdFiles is the project: a's weapons, b reading the potion directory not there yet, c every
// JSON file below a, d the potion directory through its link d/pl, dangling until then.
func createdFiles() map[string]string {
	return map[string]string{
		"project.canon":           "project acme {\n  canon: \"0.1\"\n}\n",
		"a/a.canon":               createdItemsA,
		"a/items/weapon/axe.json": "{\n  \"kind\": \"weapon\"\n}\n",
		"a/items/weapon/bow.json": "{\n  \"kind\": \"weapon\"\n}\n",
		"b/b.canon":               "/// B.\npackage b\n\n/// I.\nrecord I {\n  /// K.\n  kind: String\n}\n\n/// T.\nlet t: [I] = load.dir(\"../a/items/potion/*.json\", partial: true)\n",
		"c/c.canon":               "/// C.\npackage c\n\n/// J.\nrecord J {\n  /// K.\n  kind: String\n  /// V.\n  v: Int(..3)?\n}\n\n/// Js.\nlet js: [J] = load.dir(\"../a/**/*.json\")\n",
		"d/d.canon":               "/// D.\npackage d\n\n/// P.\nrecord P {\n  /// K.\n  kind: String\n  /// V.\n  v: Int(..2)?\n}\n\n/// Ps.\nlet ps: [P] = load.dir(\"pl/*.json\")\n",
	}
}

// createdOpen writes createdFiles under a directory of its own and opens it through a link to
// that directory when linked, as a project under /tmp or /var is opened on macOS; with the
// directory its files are under.
func createdOpen(t *testing.T, linked bool) (*Project, string) {
	t.Helper()
	top := t.TempDir()
	real := filepath.Join(top, "real")
	linksWrite(t, real, createdFiles())
	if err := os.Symlink("../a/items/potion", filepath.Join(real, "d", "pl")); err != nil {
		t.Skipf("no symbolic links here: %v", err)
	}
	dir := real
	if linked {
		dir = filepath.Join(top, "link")
		if err := os.Symlink("real", dir); err != nil {
			t.Skipf("no symbolic links here: %v", err)
		}
	}
	p, err := Open(dir, Options{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	return p, real
}

// createdCase is an edit of the walk test, whether it is accepted, adding no error to check,
// whether its Undo is dead once made with AllowErrors (E22), which refuses it so, and whether its
// Undo, applied, gives a:items back with its files.
type createdCase struct {
	name  string
	ops   []Op
	clean bool
	dead  bool
	undo  bool
}

// API.md E17, E18, E19 (DECISIONS 330): an edit of a file a load.dir walk reads, in a directory it
// creates or removes, or through a linked project, is judged as canon check then judges the files
// it leaves. b's base appears with the first potion; a's stays, emptied, when its last item goes (N6).
func TestEditCreatedUnderWalk(t *testing.T) {
	cases := []createdCase{
		{"a potion, b's base created", []Op{AddEntry("a:items", Key("pot"), Source("{ kind: potion }"))}, true, false, false},
		{"a potion over d's bound, through d's dangling link", []Op{AddEntry("a:items", Key("pot"), Source("{ kind: potion, v: 2 }"))}, false, false, false},
		{"a weapon over c's bound", []Op{AddEntry("a:items", Key("big"), Source("{ kind: weapon, v: 4 }"))}, false, false, false},
		{"a weapon over a's bound", []Op{AddEntry("a:items", Key("huge"), Source("{ kind: weapon, v: 7 }"))}, false, false, false},
		{"a weapon removed", []Op{Remove("a:items.axe")}, true, false, false},
		// API.md N6 (DECISIONS 482), WIRE.md 6.5: the walk's base stays, so a:items is empty (W7107), not E7004
		{name: "every weapon removed, a's base kept", ops: []Op{Remove("a:items.axe"), Remove("a:items.bow")}, clean: true, undo: true},
	}
	for _, linked := range []bool{false, true} {
		for _, c := range cases {
			runCreated(t, linked, c)
		}
	}
}

// runCreated runs c on a project of its own: accepted, it adds no error to canon check; refused,
// it does, once made with AllowErrors.
func runCreated(t *testing.T, linked bool, c createdCase) {
	t.Helper()
	p, dir := createdOpen(t, linked)
	ctx := context.Background()
	before := checkErrors(t, p)
	items := valueText(t, p, createdItems)
	res, err := p.Edit(ctx, Edit{Ops: c.ops})
	if c.clean != (err == nil) {
		t.Errorf("linked %v, %s: edit %v %+v, want clean %v", linked, c.name, err, res, c.clean)
		return
	}
	if !c.clean && !errors.Is(err, ErrRejected) {
		t.Errorf("linked %v, %s: %v, want ErrRejected", linked, c.name, err)
	}
	if !c.clean {
		_, err = p.Edit(ctx, Edit{Ops: c.ops, AllowErrors: true})
		switch {
		case c.dead && !errors.Is(err, ErrNoValue):
			t.Errorf("API.md E22, linked %v, %s: %v with AllowErrors, want ErrNoValue (its Undo is dead)", linked, c.name, err)
		case !c.dead && err != nil:
			t.Fatal(err)
		}
	}
	after := checkErrors(t, p)
	if added := slices.ContainsFunc(after, func(e string) bool { return !slices.Contains(before, e) }); added != (!c.clean && !c.dead) {
		t.Errorf("linked %v, %s: canon check's errors went from %v to %v", linked, c.name, before, after)
	}
	if c.undo {
		createdUndone(t, p, dir, res, items)
	}
}

// createdItems is the table createdFiles's load.dir reads.
const createdItems = "a:items"

// createdUndone checks that res, an edit removing every weapon, kept the walk's base and left
// a:items empty, and that its Undo gives the table back with both entry files (API.md N6, E22).
func createdUndone(t *testing.T, p *Project, dir string, res *EditResult, items string) {
	t.Helper()
	base := filepath.Join(dir, "a", "items")
	if list, err := os.ReadDir(base); err != nil || len(list) != 0 {
		t.Errorf("API.md N6: the walk's base a/items is %v, %v; want it kept, empty", list, err)
	}
	if got := valueText(t, p, createdItems); got == items {
		t.Errorf("API.md N6: a:items is still %s once every weapon is removed", got)
	}
	if _, err := p.Edit(context.Background(), Edit{Base: res.Revision, Ops: res.Undo}); err != nil {
		t.Fatalf("API.md E22: the Undo %v of every weapon removed: %v", res.Undo, err)
	}
	if got := valueText(t, p, createdItems); got != items {
		t.Errorf("API.md E22: a:items after the Undo is %s, want %s", got, items)
	}
	for _, name := range []string{"axe", "bow"} {
		if _, err := os.Stat(filepath.Join(base, "weapon", name+".json")); err != nil {
			t.Errorf("API.md E22, N2: the Undo did not recreate %s's file: %v", name, err)
		}
	}
}

// valueText is the canonical text of the value at path.
func valueText(t *testing.T, p *Project, path string) string {
	t.Helper()
	v, err := p.Value(context.Background(), path)
	if err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return v.Text
}

// checkErrors is each error canon check reports of p's whole project, by package and code.
func checkErrors(t *testing.T, p *Project) []string {
	t.Helper()
	res, err := p.Check(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return codesOf(errorFindings(res.Findings))
}
