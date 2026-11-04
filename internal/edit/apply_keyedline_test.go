package edit_test

import (
	"context"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/edit"
)

// keyedLineK is a package whose keyed list has a String key: keys that are words, one that is
// not, one that is a keyword; a record holding such a list below a field; lists keyed by an enum
// and by a ref, whose keys are words too.
const keyedLineK = `/// K.
package k

/// A colour.
enum Color { red, green }

/// A tone.
record Tone {
  /// Its colour.
  c: Color
  /// A count.
  n: Int = 0
}

/// An entry.
record Entry {
  /// A value.
  v: Int = 0
}

/// A link to an entry.
record Link {
  /// The entry.
  to: ref entries
  /// A count.
  n: Int = 0
}

/// Entries.
let entries: table Entry = {
  a { v: 1 }
}
/// Tones.
let tones: [Tone] keyed by c = [{ c: red }]
/// Links.
let links: [Link] keyed by to = [{ to: a }]

/// An item.
record Item {
  /// Its id.
  id: String
  /// A count.
  n: Int = 0
}

/// A box of items.
record Box {
  /// Its items.
  items: [Item] keyed by id = []
}

/// Items.
let items: [Item] keyed by id = [{ id: "a" }, { id: "a b" }, { id: "b", n: 2 }, { id: "type" }, { id: "if" }]
/// A box.
let box: Box = { items: [{ id: "a" }] }
`

// keyedLine is keyedLineK with its layers: dev amends items through a line its keyed list's word
// key starts, lz amends box.
var keyedLine = mapFS{
	"law/project.canon":     file("project acme {\n  canon: \"0.1\"\n}\n"),
	"law/k/k.canon":         file(keyedLineK),
	"law/k/dev.layer.canon": file("package k\nlayer dev\n\namend items {\n  b.n: 9\n}\n"),
	"law/k/lz.layer.canon":  file("package k\nlayer lz\n\namend box {\n  items: [{ id: \"a\", n: 1 }]\n}\n"),
}

// API.md 6.4 (DECISIONS 450; EVALUATION.md 9.2): a Set at a String-keyed keyed list's word key
// writes a new amend line the key starts, bare, in the let's block, a new block or a new file; the
// re-check is clean and the Undo restores every value and the layer file.
func TestDecisions450KeyedListWordKeyStartsLine(t *testing.T) {
	cases := []struct {
		name, editLayer, path, file, want string
		layers                            []string
	}{
		{"into the block", "dev", "k:items[a].n", "law/k/dev.layer.canon",
			"package k\nlayer dev\n\namend items {\n  b.n: 9\n  a.n: 5\n}\n", []string{"dev"}},
		{"into the block, inactive", "dev", "k:items[a].n", "law/k/dev.layer.canon",
			"package k\nlayer dev\n\namend items {\n  b.n: 9\n  a.n: 5\n}\n", nil},
		{"a new block", "lz", "k:items[a].n", "law/k/lz.layer.canon",
			"package k\nlayer lz\n\namend box {\n  items: [{ id: \"a\", n: 1 }]\n}\n\namend items { a.n: 5 }\n", []string{"lz"}},
		{"a new file", "nf", "k:items[a].n", "law/k/nf.layer.canon",
			"package k\nlayer nf\n\namend items { a.n: 5 }\n", nil},
		{"the entry", "dev", "k:items[a]", "law/k/dev.layer.canon",
			"package k\nlayer dev\n\namend items {\n  b.n: 9\n  a: { id: \"a\", n: 5 }\n}\n", []string{"dev"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			op := edit.Operation{Kind: edit.OpSet, Path: c.path, Value: edit.Int(5)}
			if c.path == "k:items[a]" {
				op.Value = edit.Source(`{ id: "a", n: 5 }`)
			}
			s := open(t, keyedLine, c.layers, c.editLayer, "k")
			plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{op}})
			if err != nil {
				t.Fatalf("Set %s: %v, want it applied (DECISIONS 450)", c.path, err)
			}
			for _, ch := range plan.Changes {
				checkWritten(t, plan, ch)
			}
			after := written(keyedLine, plan)
			if got := fileText(after, c.file); got != c.want {
				t.Errorf("%s holds:\n%s\nwant:\n%s", c.file, got, c.want)
			}
			checkKeyedLineClean(t, after, c.editLayer)
			g := goldenCase{fsys: keyedLine, pkgs: []string{"k"}, layers: c.layers, editLayer: c.editLayer}
			checkUndo(t, g, []edit.Operation{op}, plan)
			checkUndoFile(t, g, plan, c.file)
		})
	}
}

// checkKeyedLineClean opens fsys with layer active: no error finding, items[a].n is 5 (E18).
func checkKeyedLineClean(t *testing.T, fsys mapFS, layer string) {
	t.Helper()
	s := open(t, fsys, []string{layer}, "", "k")
	got := values(s, false)
	for _, f := range s.a.Bag("k").Findings() {
		if f.Severity == diag.Error {
			t.Errorf("after the edit: %s %s %s", f.Code, f.Path, f.Message)
		}
	}
	if want := `Item{id: "a", n: 5}`; !strings.Contains(got["k:items"], want) {
		t.Errorf("items = %s, want it to hold %s", got["k:items"], want)
	}
}

// checkUndoFile is API.md E22 on a layer file the edit found: the plan's Undo gives its bytes
// back (a file the edit created keeps its header: E22 restores values).
func checkUndoFile(t *testing.T, c goldenCase, plan *edit.Plan, name string) {
	t.Helper()
	if c.fsys[name] == nil {
		return
	}
	after := written(c.fsys, plan)
	s := open(t, after, c.layers, c.editLayer, c.pkgs...)
	undo, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: plan.Undo})
	if err != nil {
		t.Fatalf("Undo %v: %v", plan.Undo, err)
	}
	if got, want := fileText(written(after, undo), name), fileText(c.fsys, name); got != want {
		t.Errorf("Undo: %s holds:\n%s\nwant:\n%s", name, got, want)
	}
}

// fileText is the text of file name in fsys, empty when there is none.
func fileText(fsys mapFS, name string) string {
	if f := fsys[name]; f != nil {
		return string(f.Data)
	}
	return ""
}

// API.md 6.4 (DECISIONS 450, 456; GRAMMAR.md 3.1): Editable and Apply keep `layer` for a first key no
// word or no String (an enum); a String word key, a reserved one (`type`, `if`) too, first or below a
// field, is written, and so is a ref key into a table, whose chain ends at a table id.
func TestDecisions450KeyedLineRefusals(t *testing.T) {
	cases := []struct {
		editLayer string
		op        edit.Operation
		want      edit.Reason
	}{
		{"dev", edit.Operation{Kind: edit.OpSet, Path: `k:items["a b"].n`, Value: edit.Int(5)}, edit.ReasonLayer},
		{"dev", edit.Operation{Kind: edit.OpSet, Path: "k:items[type].n", Value: edit.Int(5)}, edit.ReasonNone},
		{"dev", edit.Operation{Kind: edit.OpSet, Path: "k:items[if].n", Value: edit.Int(5)}, edit.ReasonNone},
		{"dev", edit.Operation{Kind: edit.OpSet, Path: "k:tones[red].n", Value: edit.Int(5)}, edit.ReasonLayer},
		{"dev", edit.Operation{Kind: edit.OpSet, Path: "k:links[a].n", Value: edit.Int(5)}, edit.ReasonNone}, // DECISIONS 456
		{"dev", edit.Operation{Kind: edit.OpSet, Path: `k:items["a b"]`, Value: edit.Source(`{ id: "a b", n: 5 }`)}, edit.ReasonLayer},
		{"dev", edit.Operation{Kind: edit.OpSet, Path: "k:items[a].n", Value: edit.Int(5)}, edit.ReasonNone},
		{"dev", edit.Operation{Kind: edit.OpSet, Path: "k:box.items[a].n", Value: edit.Int(5)}, edit.ReasonNone},
	}
	for _, c := range cases {
		t.Run(c.op.Path, func(t *testing.T) {
			s := open(t, keyedLine, []string{"dev"}, c.editLayer, "k")
			f := fixture{Snapshot: s.snap, files: s.a.Files()}
			checkVerdict(t, s, resolve(t, f, c.op.Path), c.op, c.want)
		})
	}
}

// API.md 6.4 (DECISIONS 450): a keyed list's String key past a line's first step stays a string literal.
func TestDecisions450LaterKeyQuoted(t *testing.T) {
	s := open(t, keyedLine, []string{"dev"}, "dev", "k")
	op := edit.Operation{Kind: edit.OpSet, Path: "k:box.items[a].n", Value: edit.Int(5)}
	plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{op}})
	if err != nil {
		t.Fatal(err)
	}
	want := "package k\nlayer dev\n\namend items {\n  b.n: 9\n}\n\namend box { items[\"a\"].n: 5 }\n"
	if got := string(written(keyedLine, plan)["law/k/dev.layer.canon"].Data); got != want {
		t.Errorf("dev.layer.canon holds:\n%s\nwant:\n%s", got, want)
	}
}
