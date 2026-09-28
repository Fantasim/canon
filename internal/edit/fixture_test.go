package edit_test

import (
	"context"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
)

// mapFS is an in-memory project tree for the rows the examples do not reach.
type mapFS fstest.MapFS

func rel(name string) string { return strings.TrimPrefix(name, "/") }

func (m mapFS) ReadFile(name string) ([]byte, error)       { return fstest.MapFS(m).ReadFile(rel(name)) }
func (m mapFS) Stat(name string) (fs.FileInfo, error)      { return fstest.MapFS(m).Stat(rel(name)) }
func (m mapFS) ReadDir(name string) ([]fs.DirEntry, error) { return fstest.MapFS(m).ReadDir(rel(name)) }

func file(text string) *fstest.MapFile { return &fstest.MapFile{Data: []byte(text)} }

// lawA is package a of law: one source of each kind API.md §6-§7 names.
const lawA = `/// A.
package a

/// A colour.
enum Color { red, green = "GREEN" }

/// A shape.
variant Shape {
  /// A circle.
  circle {
    /// Radius.
    r: Int
  }
  /// A square.
  square {
    /// Side.
    side: Int = 1
  }
}

/// An item.
record Item {
  /// Its id.
  id: String
  /// A count.
  n: Int = 0
}

/// A row.
record Row {
  /// Its code.
  code: Int
  /// Its label.
  label: String = ""
}

/// A base.
record Base {
  /// X.
  x: Int = 1
  /// Y.
  y: Int = 2
  /// A sub item.
  sub: Item = { id: "s" }
}

/// An entry.
record Entry {
  /// A value.
  v: Int = 0
}

/// Holds a loaded item.
record Holder {
  /// The item.
  data: Item
}

/// A tone per colour.
record Tone {
  /// Its colour.
  c: Color
  /// A weight.
  w: Int = 0
}

/// A link to an entry.
record Link {
  /// The entry.
  to: ref entries
  /// A weight.
  w: Int = 0
}

/// Shared.
let shared: Int = 1
/// Per colour.
let byColor: {Color: Int} = { red: 1, green: 2 }
/// String keys.
let named: {String: Int} = { "plain": 1, "two words": 2, "say \"hi\"": 3 }
/// Integer keys.
let numbered: {Int: String} = { 3: "three", -4: "minus four" }
/// Items.
let items: [Item] keyed by id = [{ id: "a" }, { id: "b", n: 2 }]
/// Rows.
let rows: [Row] keyed by code = [{ code: 7 }]
/// Tones.
let tones: [Tone] keyed by c = [{ c: green }]
/// Links.
let links: [Link] keyed by to = [{ to: one }]
/// A shape.
let shape: Shape = circle { r: 3 }
/// A base.
let base: Base = { x: 5 }
/// A derived base.
let derived: Base = { ...base, y: 9 }
/// Computed.
let computed: Int = shared + 1
/// Read through a name.
let viaName: Item = items.a
/// A list with a name in it.
let mixed: [Int] = [shared, 2]
/// A record with a name in it.
let wrap: Row = { code: shared, label: "w" }
/// A table with an entry declared apart.
let entries: table Entry = {
  one { v: 1 }
}
/// Per entry.
let perEntry: {e in entries: Int} = { one: 1, two: 2 }
/// A loaded item.
let holder: Holder = { data: load("item.json") }
/// Notes.
let notes: String = load.text("notes.txt")
/// Constant keys.
const KS = [1, 2]
/// Hidden.
local let hidden: Int = 3
/// Defines.
local let defs = load.defines("defs.h")
/// Another holder.
let other: Holder = { data: { id: "o" } }
/// A bare case.
let sq: Shape = square

/// Entries, each in its own file.
@files("filed/{id}.canon")
let filed: table Entry = {}

/// A key kind.
enum Mode { colour, shape }

/// The key type a mode picks.
type Pick(m: Mode) = match m {
  colour => Color
  shape => Mode
}

/// A map keyed by a dependent type.
record Picked {
  /// The mode.
  m: Mode
  /// Weights per key.
  w: {Pick(m): Int}
}

/// Weights per colour.
let picked: Picked = { m: colour, w: { red: 1, green: 2 } }

/// A bag of collections.
record Bag {
  /// Per colour.
  byC: {Color: Int}
  /// Numbers.
  xs: [Int]
  /// Tones.
  ts: [Tone] keyed by c
  /// A base.
  b: Base = {}
}

/// A bag.
let bag: Bag = { byC: { red: 1, green: 2 }, xs: [1, 2, 3], ts: [{ c: red }] }

/// Makes an item.
local fn mk() -> Item { return { id: "m" } }
`

// law is a small project: a (above), b (names that clash with a's), u (a literal-union key).
var law = mapFS{
	"law/project.canon":       file("project acme {\n  canon: \"0.1\"\n}\n"),
	"law/a/a.canon":           file(lawA),
	"law/a/Extra/more.canon":  file("package a\n\n/// Two.\nentry entries.two { v: 2 }\n"),
	"law/a/dev.layer.canon":   file("package a\nlayer dev\n\namend wrap {\n  code: 5\n  label: \"v\"\n}\n\namend base {\n  x: 7\n  sub: mk()\n}\n\namend holder {\n  data: { id: \"L\", n: 1 }\n}\n\namend other {\n  data: load(\"item.json\")\n}\n\namend defs {\n  A.value: 5\n}\n"),
	"law/a/stage.layer.canon": file("package a\nlayer stage\n\namend items {\n  a: { id: \"a\", n: 3 }\n  b.n: 9\n}\n"),
	"law/a/lc.layer.canon":    file("package a\nlayer lc\n\namend holder {\n  data: mk()\n}\n\namend other {\n  data: load(\"item.json\")\n}\n"),
	"law/a/ld.layer.canon":    file("package a\nlayer ld\n\namend bag {\n  byC[Color.green]: 7\n  xs[-1]: 9\n  xs[#0]: 8\n}\n"),
	"law/a/le.layer.canon":    file("package a\nlayer le\n\namend bag {\n  byC: { red: 4, green: 5 }\n  xs: [7, 8, 9]\n  ts: [{ c: red, w: 6 }]\n}\n"),
	"law/a/la.layer.canon":    file("package a\nlayer la\n\namend holder {\n  data: { id: \"A\", n: 2 }\n}\n"),
	"law/a/lb.layer.canon":    file("package a\nlayer lb\n\namend holder {\n  data.n: 3\n}\n"),
	"law/a/lf.layer.canon":    file("package a\nlayer lf\n\namend bag {\n  xs: [5]\n  ts: [{ c: green, w: 1 }]\n  byC: { red: 3 }\n  b: { ...base, y: 1 }\n}\n\namend other {\n  data: { id: \"q\" }\n}\n"),
	"law/a/a/x.canon":         file("package a\n\n/// Deep.\nlet deep: Int = 1\n"),
	"law/a/defs.h":            file("#define A 1\n#define B 2\n"),
	"law/a/item.json":         file("{\"id\": \"z\", \"n\": 5}\n"),
	"law/a/notes.txt":         file("hello\n"),
	"law/b/b.canon":           file("/// B.\npackage b\n\n/// Shared too.\nlet shared: Int = 2\n\n/// Named like an enum.\nlet Color: Int = 0\n"),
	"law/u/u.canon": file("/// U.\npackage u\n\nimport a { Color }\n\n/// A literal union key.\n" +
		"let tagged: {Color | \"none\": Int} = { red: 1, \"none\": 0 }\n"),
}

// fixture is a snapshot and the files its spans point into.
type fixture struct {
	*edit.Snapshot
	files diag.Files
}

// text is the source text a span covers.
func (f fixture) text(sp source.Span) string {
	return string(f.files.Content(sp.File)[sp.Start:sp.End])
}

// examples is the example project with pkgs selected and layers active, its roots redirected
// as internal/build's tests do.
func examples(t *testing.T, layers []string, pkgs ...string) fixture {
	t.Helper()
	dir, err := filepath.Abs("../../examples")
	if err != nil {
		t.Fatal(err)
	}
	roots := map[string]string{"resource": "_fixtures/resource", "client": "_fixtures/client"}
	out := t.TempDir()
	for _, name := range []string{"source", "services", "sovcommon", "web", "parity", "generated"} {
		roots[name] = filepath.ToSlash(filepath.Join(out, name))
	}
	p, err := build.Open(project.OS(), filepath.ToSlash(dir), build.Options{Roots: roots, Layers: layers})
	if err != nil {
		t.Fatal(err)
	}
	return analyze(t, p, pkgs, true)
}

// buildOpen opens a project rooted at /law of fsys.
func buildOpen(fsys mapFS) (*build.Project, error) {
	return build.Open(fsys, "/law", build.Options{})
}

// lawFixture is law with pkgs selected and layers active.
func lawFixture(t testing.TB, layers []string, pkgs ...string) fixture {
	t.Helper()
	p, err := build.Open(law, "/law", build.Options{Layers: layers})
	if err != nil {
		t.Fatal(err)
	}
	return analyze(t, p, pkgs, true)
}

func analyze(t testing.TB, p *build.Project, pkgs []string, clean bool) fixture {
	t.Helper()
	a, err := p.Analyze(context.Background(), pkgs)
	if err != nil {
		t.Fatal(err)
	}
	if clean && a.Result().Summary.Errors > 0 {
		t.Fatalf("%v: %d errors: %v", pkgs, a.Result().Summary.Errors, a.Result().List)
	}
	return fixture{edit.NewSnapshot(a), a.Files()}
}

// resolve is Resolve of the path text, failing the test on any error.
func resolve(t *testing.T, f fixture, path string) edit.Resolved {
	t.Helper()
	p, err := edit.Parse(path)
	if err != nil {
		t.Fatalf("Parse(%q): %v", path, err)
	}
	r, err := edit.Resolve(f.Snapshot, p)
	if err != nil {
		t.Fatalf("Resolve(%q): %v", path, err)
	}
	return r
}

// mustEdit is Editable, failing the test on an error.
func mustEdit(t testing.TB, f fixture, r edit.Resolved, op edit.Op, layer string) edit.Editability {
	t.Helper()
	e, err := f.Editable(r, op, layer)
	if err != nil {
		t.Fatalf("Editable(%s, %d, %q): %v", r.Canonical, op, layer, err)
	}
	return e
}

// resolveErr is Resolve's refusal of the path text.
func resolveErr(t *testing.T, f fixture, path string) error {
	t.Helper()
	p, err := edit.Parse(path)
	if err != nil {
		t.Fatalf("Parse(%q): %v", path, err)
	}
	r, err := edit.Resolve(f.Snapshot, p)
	if err == nil {
		t.Fatalf("Resolve(%q) = %s, want an error", path, r.Canonical)
	}
	return err
}
