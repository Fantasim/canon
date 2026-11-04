package edit_test

import (
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// baseLayeredSrc is a package whose records a dev layer amends at every depth: a let, a record
// below a defaulted field, a map entry and a table entry; url's default reads the port.
const baseLayeredSrc = `package d

/// A server.
record Server {
  /// Port.
  port: Int = 1
  /// Host.
  host: String = "localhost"
  /// URL.
  url: String = "http://{host}:{port}"
}

/// A node.
record Node {
  /// Its server.
  server: Server = {}
  /// Tag.
  tag: String = "t"
}

/// A fleet.
record Fleet {
  /// Its nodes, by name.
  nodes: {String: Node} = {}
}

/// The server.
let server: Server = {}

/// The written server.
let named: Server = { host: "h" }

/// The node.
let node: Node = {}

/// The fleet.
let fleet: Fleet = { nodes: { "a": {} } }

/// The rows.
let rows: table Node = {
  a {}
}

/// A bag.
record Bag {
  /// Numbers.
  ns: [Int] = [1, 2]
  /// Servers.
  items: [Server] = []
}

/// The bag.
let bag: Bag = {}

/// The crew.
let crew: Bag = { items: [{ host: "a" }, { host: "b" }] }

/// A shape.
variant Shape {
  /// A circle.
  circle {
    /// Radius.
    r: Int = 1
    /// Tag.
    tag: String = "c"
  }
  /// A square.
  square {
    /// Side.
    side: Int = 1
    /// Tag.
    tag: String = "s"
  }
}

/// A drawing.
record Drawing {
  /// Its shape.
  shape: Shape = circle {}
}

/// The drawing.
let drawing: Drawing = { shape: circle {} }
`

// baseLayeredDev amends a field of each value baseLayeredSrc declares, never the edited one.
const baseLayeredDev = `package d
layer dev

amend server {
  port: 9000
}

amend named {
  port: 9000
}

amend node {
  server.port: 9000
}

amend fleet {
  nodes["a"].server.port: 9000
}

amend rows {
  a.server.port: 9000
}

amend bag {
  ns[0]: 5
}

amend crew {
  items[0].port: 9000
}

amend drawing {
  shape.tag: "dev"
}
`

const (
	baseLayeredFile = "law/d/d.canon"
	baseLayeredDevF = "law/d/dev.layer.canon"
)

func baseLayeredFS() mapFS {
	return mapFS{"law/project.canon": file(projectCanon), baseLayeredFile: file(baseLayeredSrc), baseLayeredDevF: file(baseLayeredDev)}
}

// baseLayeredCase is a base edit, the one line of the base file it must turn into ("" removes it),
// and that line once undone when it is not old: a defaulted parent the Undo empties may stay `{}`
// (API.md E22).
type baseLayeredCase struct {
	name             string
	op               edit.Operation
	old, new, undone string
}

var baseLayeredCases = []baseLayeredCase{
	{"url (W7)", edit.Operation{Kind: edit.OpSet, Path: "d:server.url", Value: edit.Str("x")},
		`let server: Server = {}`, `let server: Server = { url: "x" }`, ""},
	{"host (W7)", edit.Operation{Kind: edit.OpSet, Path: "d:server.host", Value: edit.Str("x")},
		`let server: Server = {}`, `let server: Server = { host: "x" }`, ""},
	{"reset beside an amended field", edit.Operation{Kind: edit.OpReset, Path: "d:named.host"},
		`let named: Server = { host: "h" }`, `let named: Server = {}`, ""},
	{"nested record (W8)", edit.Operation{Kind: edit.OpSet, Path: "d:node.server.host", Value: edit.Str("x")},
		`let node: Node = {}`, `let node: Node = { server: { host: "x" } }`, `let node: Node = { server: {} }`},
	{"sibling of an amended record", edit.Operation{Kind: edit.OpSet, Path: "d:node.tag", Value: edit.Str("x")},
		`let node: Node = {}`, `let node: Node = { tag: "x" }`, ""},
	{"map entry (W8)", edit.Operation{Kind: edit.OpSet, Path: "d:fleet.nodes[a].server.host", Value: edit.Str("x")},
		`let fleet: Fleet = { nodes: { "a": {} } }`, `let fleet: Fleet = { nodes: { "a": { server: { host: "x" } } } }`,
		`let fleet: Fleet = { nodes: { "a": { server: {} } } }`},
	{"table entry (W8)", edit.Operation{Kind: edit.OpSet, Path: "d:rows.a.server.host", Value: edit.Str("x")},
		"  a {}", `  a { server: { host: "x" } }`, "  a { server: {} }"},
	{"whole record", edit.Operation{Kind: edit.OpSet, Path: "d:named", Value: edit.Source(`{ host: "y" }`)},
		`let named: Server = { host: "h" }`, `let named: Server = { host: "y" }`, ""},
	{"add to a defaulted list (W8)", edit.Operation{Kind: edit.OpAdd, Path: "d:bag.ns", Value: edit.Int(3)},
		`let bag: Bag = {}`, `let bag: Bag = { ns: [1, 2, 3] }`, ""},
	{"remove an element a layer amends below", edit.Operation{Kind: edit.OpRemove, Path: "d:crew.items[0]"},
		`let crew: Bag = { items: [{ host: "a" }, { host: "b" }] }`, `let crew: Bag = { items: [{ host: "b" }] }`, ""},
	{"case change beside an amended field (E14)", edit.Operation{Kind: edit.OpSetCase, Path: "d:drawing.shape", Case: "square"},
		`let drawing: Drawing = { shape: circle {} }`, `let drawing: Drawing = { shape: square { tag: "c" } }`,
		`let drawing: Drawing = { shape: circle }`},
}

// API.md W8, W10, E22 (EDIT-MATERIALIZE ruling): a base edit while dev is active writes only the
// base's values, never dev's, and leaves dev's file alone; its Undo brings every value back with
// dev active and inactive, and both files byte for byte (an emptied parent as `{}`, E22).
func TestBaseEditUnderLayerWritesBase(t *testing.T) {
	for _, c := range baseLayeredCases {
		baseLayeredIn(t, c, undoView{[]string{"dev"}, ""})
	}
}

// API.md W7, W8, E23: the same edits with no layer active write the same base text (unchanged
// from before the ruling), their Undo the file back.
func TestBaseEditNoLayerWritesBase(t *testing.T) {
	for _, c := range baseLayeredCases {
		baseLayeredIn(t, c, undoView{})
	}
}

// baseLayeredIn applies c in view v: the base file changes by c's one line and dev's file not at
// all; the Undo then brings every value back with dev active and inactive, and both files byte for
// byte, and the Undo of the Undo every value of the edit.
func baseLayeredIn(t *testing.T, c baseLayeredCase, v undoView) {
	t.Helper()
	name := c.name
	if len(v.layers) > 0 {
		name += ", dev active"
	}
	fsys := baseLayeredFS()
	p1, f1, ok := applyIn(t, name, fsys, v, []edit.Operation{c.op})
	if !ok {
		return
	}
	baseLayeredFiles(t, name, f1, c, c.new)
	p2, f2, ok := applyIn(t, name+", Undo", f1, v, p1.Undo)
	if !ok {
		return
	}
	for _, ls := range [][]string{{"dev"}, nil} {
		sameIn(t, name+", Undo", fsys, f2, ls)
	}
	undone := c.old
	if c.undone != "" {
		undone = c.undone
	}
	baseLayeredFiles(t, name+", Undo", f2, c, undone)
	if _, f3, ok := applyIn(t, name+", Undo of the Undo", f2, v, p2.Undo); ok {
		for _, ls := range [][]string{{"dev"}, nil} {
			sameIn(t, name+", Undo of the Undo", f1, f3, ls)
		}
	}
}

// baseLayeredFiles checks got byte for byte: the base file is baseLayeredSrc with c's old line
// read as line ("" for none), dev's file is baseLayeredDev.
func baseLayeredFiles(t *testing.T, name string, got mapFS, c baseLayeredCase, line string) {
	t.Helper()
	if line != "" {
		line += "\n" // "" deletes the line
	}
	if want := strings.Replace(baseLayeredSrc, c.old+"\n", line, 1); string(got[baseLayeredFile].Data) != want {
		t.Errorf("%s: the base file changes by\n%swant\n%s", name, matrixHunk(baseLayeredSrc, string(got[baseLayeredFile].Data)), matrixHunk(baseLayeredSrc, want))
	}
	if string(got[baseLayeredDevF].Data) != baseLayeredDev {
		t.Errorf("%s: dev's file changes by\n%s", name, matrixHunk(baseLayeredDev, string(got[baseLayeredDevF].Data)))
	}
}

// API.md E6, W8 (EDIT-MATERIALIZE ruling): url's default reads the port dev amends, so base and
// session give two defaults; a base Set of url to either is written (materialized too), never left
// to one view's default, and url holds the value set with dev active and not.
func TestBaseSetDefaultInBothViews(t *testing.T) {
	const low, high = "http://localhost:1", "http://localhost:9000"
	cases := []struct {
		root, path, url, old, new, undone string
	}{
		{"d:server", "d:server.url", low, `let server: Server = {}`, `let server: Server = { url: "` + low + `" }`, ""},
		{"d:server", "d:server.url", high, `let server: Server = {}`, `let server: Server = { url: "` + high + `" }`, ""},
		{"d:node", "d:node.server.url", low, `let node: Node = {}`, `let node: Node = { server: { url: "` + low + `" } }`, `let node: Node = { server: {} }`},
	}
	for _, tc := range cases {
		op := edit.Operation{Kind: edit.OpSet, Path: tc.path, Value: edit.Str(tc.url)}
		c := baseLayeredCase{tc.path + " " + tc.url, op, tc.old, tc.new, tc.undone}
		baseLayeredIn(t, c, undoView{[]string{"dev"}, ""})
		_, f1, ok := applyIn(t, c.name, baseLayeredFS(), undoView{[]string{"dev"}, ""}, []edit.Operation{op})
		for _, ls := range [][]string{{"dev"}, nil} {
			if got := values(open(t, f1, ls, "", "d"), false)[tc.root]; ok && !strings.Contains(got, `url: "`+tc.url+`"`) {
				t.Errorf("%s, layers %v: %s = %s, want url %q", c.name, ls, tc.root, got, tc.url)
			}
		}
	}
}
