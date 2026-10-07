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
// that directory when linked, as a project under /tmp or /var is opened on macOS.
func createdOpen(t *testing.T, linked bool) *Project {
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
	return p
}

// createdCase is an edit of the walk test, and whether it is accepted, adding no error to check.
type createdCase struct {
	name  string
	ops   []Op
	clean bool
}

// API.md E17, E18, E19 (DECISIONS 330): an edit of a file a load.dir walk reads, in a directory it
// creates or removes, or through a linked project, is judged as canon check then judges the files
// it leaves. b's base appears with the first potion; a's goes with its last item.
func TestEditCreatedUnderWalk(t *testing.T) {
	cases := []createdCase{
		{"a potion, b's base created", []Op{AddEntry("a:items", Key("pot"), Source("{ kind: potion }"))}, true},
		{"a potion over d's bound, through d's dangling link", []Op{AddEntry("a:items", Key("pot"), Source("{ kind: potion, v: 2 }"))}, false},
		{"a weapon over c's bound", []Op{AddEntry("a:items", Key("big"), Source("{ kind: weapon, v: 4 }"))}, false},
		{"a weapon over a's bound", []Op{AddEntry("a:items", Key("huge"), Source("{ kind: weapon, v: 7 }"))}, false},
		{"a weapon removed", []Op{Remove("a:items.axe")}, true},
		{"every weapon removed, a's base with them", []Op{Remove("a:items.axe"), Remove("a:items.bow")}, false},
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
	p := createdOpen(t, linked)
	ctx := context.Background()
	before := checkErrors(t, p)
	res, err := p.Edit(ctx, Edit{Ops: c.ops})
	if c.clean != (err == nil) {
		t.Errorf("linked %v, %s: edit %v %+v, want clean %v", linked, c.name, err, res, c.clean)
		return
	}
	if !c.clean && !errors.Is(err, ErrRejected) {
		t.Errorf("linked %v, %s: %v, want ErrRejected", linked, c.name, err)
	}
	if !c.clean {
		if _, err = p.Edit(ctx, Edit{Ops: c.ops, AllowErrors: true}); err != nil {
			t.Fatal(err)
		}
	}
	after := checkErrors(t, p)
	if added := slices.ContainsFunc(after, func(e string) bool { return !slices.Contains(before, e) }); added == c.clean {
		t.Errorf("linked %v, %s: canon check's errors went from %v to %v", linked, c.name, before, after)
	}
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
