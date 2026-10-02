package canon_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/internal/diag"
)

// renameLaw is a small project of the refusals: a function a rename could make capture a
// built-in's call, a variant whose two cases declare `count` that a `@files` template names, a
// @codes enum and a @stable field.
var renameLaw = map[string]string{
	"project.canon": "project acme {\n  canon: \"0.1\"\n}\n",
	"a/a.canon": `/// A.
package a

/// Limits x.
fn limit(x: Int, lo: Int, hi: Int) -> Int {
  return x
}

/// A value.
let v: Int = clamp(5, 0, 10)

/// A prize.
variant Prize {
  /// Gold.
  gold {
    /// How many.
    count: Int
  }
  /// A gem.
  gem {
    /// How many.
    count: Int
  }
}

/// An item.
record Item {
  /// Its prize.
  prize: Prize
}

/// The items, by the count of their prize.
@files("items/{prize.count}/{id}.canon")
let items: table Item = {
  a { prize: gold { count: 1 } }
}

/// An element.
enum Element @codes(UInt8) { fire = 1, water = 2 }

/// A code.
record Code {
  /// Its tag.
  tag: String @stable
}

/// The codes.
let codes: stable table Code = {
  first { tag: "f" }
}
`,
	"a/canon.lock": "# canon.lock v1\nenum   a.Element  1  fire\nenum   a.Element  2  water\nfield  a.codes.tag  \"f\"  first\ntable  a.codes  first\n",
}

// openRename opens renameLaw with the changes given.
func openRename(t *testing.T, changes map[string]string) (*canon.Project, *memFS) {
	t.Helper()
	law := map[string]string{}
	for name, text := range renameLaw { //canon:unordered each file is copied under its own name
		law[name] = text
	}
	for name, text := range changes { //canon:unordered each file is copied under its own name
		law[name] = text
	}
	opts := project(law)
	p, err := canon.Open("/law", opts)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = p.Close() })
	m, _ := opts.FS.(*memFS)
	return p, m
}

// API.md E35, API.md E21: a call of the built-in clamp captured by a function renamed `clamp` is
// ErrNameClash naming the call and what it would name, DryRun included; nothing is written.
func TestRenameNameCapture(t *testing.T) {
	p, m := openRename(t, nil)
	before := read(t, m, "a/a.canon")
	for _, dry := range []bool{false, true} {
		e := canon.Edit{Ops: []canon.Op{canon.RenameName("a:limit", "clamp")}, DryRun: dry}
		_, err := p.Edit(context.Background(), e)
		var pe *canon.PathError
		if !errors.Is(err, canon.ErrNameClash) || !errors.As(err, &pe) || pe.Op != 0 || pe.Path != "a:limit" {
			t.Fatalf("API.md E35, dry run %v: %v", dry, err)
		}
		if want := "a/a.canon:10:14 would name a:clamp, declared at a/a.canon:5:4"; pe.Detail != want {
			t.Errorf("API.md E35: detail %q, want %q", pe.Detail, want)
		}
	}
	if read(t, m, "a/a.canon") != before {
		t.Error("API.md E35: a refused rename wrote")
	}
}

// API.md E35: an occurrence the index marks ambiguous (`{prize.count}` with two cases declaring
// `count`) refuses renaming either case's field with ErrNameClash, its Detail naming it.
func TestRenameNameAmbiguousOccurrence(t *testing.T) {
	p, _ := openRename(t, nil)
	for _, name := range []string{"a:Prize.gold.count", "a:Prize.gem.count"} {
		_, err := renameOnce(p, name, "amount")
		var pe *canon.PathError
		if !errors.Is(err, canon.ErrNameClash) || !errors.As(err, &pe) || !strings.HasPrefix(pe.Detail, "a/a.canon:33:") {
			t.Errorf("API.md E35, %s: %v", name, err)
		}
	}
}

// API.md E35, API.md E19: a rename whose re-check finds a collision is refused with the
// re-check's findings, ErrRejected, before the capture check.
func TestRenameNameCollision(t *testing.T) {
	dir := copyRenames(t)
	before := tree(t, dir)
	p := openDisk(t, dir, canon.Options{})
	res, err := renameOnce(p, "features.renames:Mission.name", "level")
	var rej *canon.RejectedError
	if !errors.As(err, &rej) || res == nil || res.Applied {
		t.Fatalf("API.md E35: %v", err)
	}
	if len(rej.Findings) == 0 || rej.Findings[0].Code != string(diag.E2106.Def().Code) {
		t.Errorf("API.md E35: findings %+v, want the duplicate field", rej.Findings)
	}
	if got := changedFiles(before, tree(t, dir)); len(got) > 0 {
		t.Errorf("API.md E19: a rejected rename wrote %v", got)
	}
}

// API.md E29, API.md E28: a @codes enum, a @stable field and an entry of a stable table are
// named by canon.lock: ErrStableKey; a built-in, named by a position on its call, is ErrBadOp.
func TestRenameNameLocked(t *testing.T) {
	p, _ := openRename(t, nil)
	cases := []struct {
		name string
		want error
	}{
		{"a:Element", canon.ErrStableKey},
		{"a:Element.fire", canon.ErrStableKey},
		{"a:Code.tag", canon.ErrStableKey},
		{"a:codes", canon.ErrStableKey},
		{"a:codes.first", canon.ErrStableKey},
		{"a/a.canon:10:14", canon.ErrBadOp},
	}
	for _, c := range cases {
		if _, err := renameOnce(p, c.name, "other"); !errors.Is(err, c.want) {
			t.Errorf("API.md E28, E29, %s: %v, want %v", c.name, err, c.want)
		}
	}
}

// API.md E32, API.md E21: broken declarations of the package or an importer refuse the rename,
// reason broken, listed as `<file>:<line>`; for a parameter only its function counts (the
// re-check then refuses it); the new name is judged before the base.
func TestRenameNameBrokenBase(t *testing.T) {
	broken := renameLaw["a/a.canon"] + "\n/// Broken.\nlet bad: Int = \"x\"\n"
	p, _ := openRename(t, map[string]string{
		"a/a.canon": broken,
		"b/b.canon": "/// B.\npackage b\n\nimport a\n\n/// Also broken.\nlet worse: Int = a.v + \"y\"\n",
	})
	_, err := renameOnce(p, "a:Item", "Thing")
	var ne *canon.NotEditableError
	if !errors.As(err, &ne) || ne.Reason != canon.ReasonBroken || ne.Detail != "a/a.canon:53, b/b.canon:7" || ne.Op != 0 {
		t.Errorf("API.md E32: %v", err)
	}
	if _, err := renameOnce(p, "a:Item", "check"); !errors.Is(err, canon.ErrBadValue) {
		t.Errorf("API.md E21: the new name before the base: %v", err)
	}
	if _, err := renameOnce(p, "a:limit.hi", "top"); errors.As(err, &ne) || !errors.Is(err, canon.ErrRejected) {
		t.Errorf("API.md E32: a parameter of a sound function: %v", err)
	}
}

// renameData is a package of data names: an enum, a variant, a table, a keyed list and a map.
const renameData = `/// C.
package c

/// A hue.
enum Hue { red, blue }

/// A shape.
variant Shape {
  /// A dot.
  dot
  /// A ring.
  ring
}

/// A person.
record Person {
  /// The name.
  name: String
}

/// The paints.
let paints: table Person = {
  wall { name: "w" }
}

/// The people.
let people: [Person] keyed by name = [{ name: "ann" }]

/// The limits.
let limits: {String: Int} = { "x": 1 }
`

// API.md E28 (DECISIONS 277): data is ErrBadOp; an enum member's or a variant case's detail says
// it cannot be renamed, an entry key's, a keyed-list key's and a map key's names Rename.
func TestRenameNameDataDetail(t *testing.T) {
	p, _ := openRename(t, map[string]string{"c/c.canon": renameData})
	cases := []struct {
		name, want, not string
	}{
		{"c:Hue.red", "an enum member or a variant case cannot be renamed", "Rename"},
		{"c:Shape.dot", "an enum member or a variant case cannot be renamed", "Rename"},
		{"c:paints.wall", "a key changes with Rename", "cannot be renamed"},
		{"c:people.ann", "a key changes with Rename", "cannot be renamed"},
		{"c:limits.x", "a key changes with Rename", "cannot be renamed"},
	}
	for _, c := range cases {
		_, err := renameOnce(p, c.name, "other")
		var pe *canon.PathError
		if !errors.Is(err, canon.ErrBadOp) || !errors.As(err, &pe) {
			t.Errorf("API.md E28, %s: %v, want ErrBadOp", c.name, err)
			continue
		}
		if !strings.Contains(pe.Detail, c.want) || strings.Contains(pe.Detail, c.not) {
			t.Errorf("API.md E28, %s: detail %q, want %q without %q", c.name, pe.Detail, c.want, c.not)
		}
	}
}
