package edit_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// emptySrc is a record whose record fields default to `{}`, beside a table a load may give, and a
// record holding a field whose default reads a record parameter.
const emptySrc = "/// P.\npackage p\n\n/// A cell.\nrecord C {\n  /// V.\n  v: Int = 0\n}\n\n" +
	"/// A box.\nrecord B {\n  /// N.\n  n: Int = 0\n  /// A cell.\n  c: C = {}\n}\n\n" +
	"/// An E0.\nrecord E0 {\n  /// W.\n  w: Int = 0\n}\n\n" +
	"/// Bound by m.\nrecord P(m: Bool) {\n  /// X.\n  x: Bool = m\n}\n\n" +
	"/// H.\nrecord H {\n  /// Rows.\n  s: table C = {}\n  /// A flag.\n  flag: Bool = true\n  /// Bound.\n  p: P(flag) = {}\n}\n\n/// Z.\nrecord Z {\n  /// H.\n  h: H = {}\n}\n\n" +
	"/// A.\nrecord A {\n  /// Rows.\n  s: table C = {}\n  /// An E0.\n  e: E0 = {}\n  /// A box.\n  b: B = {}\n  /// K.\n  k: Int = 0\n}\n\n" +
	"/// Y.\nrecord Y {\n  /// A.\n  a: A = {}\n}\n\n/// An E0.\nlet e1: E0 = { w: 4 }\n\n/// The holder.\nlet y: Y = {}\n\n/// A bound holder.\nlet z: Z = {}\n"

// The empty law's layer files.
const (
	liFile  = "law/p/li.layer.canon"
	llFile  = "law/p/ll.layer.canon"
	lbFile  = "law/p/lb.layer.canon"
	lcbFile = "law/p/lc.layer.canon"
	lpFile  = "law/p/lp.layer.canon"
)

// emptyLines are the empty law's layers: beside an identifier (li) and a load (ll), the box left
// out; the box stated beside each (lb, lc); a bound field stated beside a load (lp).
var emptyLines = map[string][2]string{
	liFile:  {"y", "a: { e: e1, k: 3 }"},
	llFile:  {"y", `a: { s: load("s.json"), k: 3 }`},
	lbFile:  {"y", "a: { e: e1, k: 3, b: { n: 2 } }"},
	lcbFile: {"y", `a: { s: load("s.json"), b: { n: 2 } }`},
	lpFile:  {"z", `h: { s: load("s.json"), p: { x: false } }`},
}

// emptyLaw is emptySrc with emptyLines.
func emptyLaw() map[string]mapFS {
	fsys := mapFS{"law/project.canon": file(projectCanon), "law/p/p.canon": file(emptySrc), "law/p/s.json": file("{\"one\": {\"v\": 1}}\n")}
	for name, line := range emptyLines { //canon:unordered fills a map
		layer := name[len("law/p/") : len(name)-len(".layer.canon")]
		fsys[name] = file(lineFile("p", layer, line[0], line[1]))
	}
	return map[string]mapFS{"empty": fsys}
}

// emptySet is a Set of path to v.
func emptySet(path string, v edit.Lit) edit.Operation {
	return edit.Operation{Kind: edit.OpSet, Path: path, Value: v}
}

// emptyRows are TestLayerParityEmptyDefault's cases: a record field whose default is `{}`, at it and
// below it, Set and Reset, left out and stated, beside an identifier and beside a load.
var emptyRows = []parityRow{
	{"left out beside an identifier: Set below", "empty", "li", liFile, inBoth, emptySet("p:y.a.b.n", edit.Int(5)), edit.ReasonNone, ""},
	// Three nested braces break the line (FORMATTER.md 7); the Undo's removal leaves it broken.
	{"left out beside an identifier: Set two below", "empty", "li", liFile, inBoth, emptySet("p:y.a.b.c.v", edit.Int(5)), edit.ReasonNone,
		lineFile("p", "li", "y", "a: {\n    e: e1\n    k: 3\n  }")},
	{"left out beside an identifier: Set at it", "empty", "li", liFile, inBoth, setTo("p:y.a.b", "{ n: 5 }"), edit.ReasonNone, ""},
	{"left out beside an identifier: Set at it to its default", "empty", "li", liFile, inBoth, setTo("p:y.a.b", "{}"), edit.ReasonNone, ""},
	{"left out beside an identifier: Reset at it", "empty", "li", liFile, inBoth, wayOp(edit.OpReset, "p:y.a.b"), edit.ReasonNone, ""},
	{"left out beside a load: Set below", "empty", "ll", llFile, inBoth, emptySet("p:y.a.b.n", edit.Int(5)), edit.ReasonNone, ""},
	{"left out beside a load: Set two below", "empty", "ll", llFile, inBoth, emptySet("p:y.a.b.c.v", edit.Int(5)), edit.ReasonNone,
		lineFile("p", "ll", "y", "a: {\n    s: load(\"s.json\")\n    k: 3\n  }")},
	{"left out beside a load: Set at it", "empty", "ll", llFile, inBoth, setTo("p:y.a.b", "{ n: 5 }"), edit.ReasonNone, ""},
	{"left out beside a load: Reset at it", "empty", "ll", llFile, inBoth, wayOp(edit.OpReset, "p:y.a.b"), edit.ReasonNone, ""},
	{"left out beside a load: Set below another", "empty", "ll", llFile, inBoth, emptySet("p:y.a.e.w", edit.Int(5)), edit.ReasonNone, ""},
	{"left out beside a load: Set at another", "empty", "ll", llFile, inBoth, setTo("p:y.a.e", "{ w: 5 }"), edit.ReasonNone, ""},
	{"stated beside an identifier: Set below", "empty", "lb", lbFile, inBoth, emptySet("p:y.a.b.n", edit.Int(5)), edit.ReasonNone, ""},
	{"stated beside an identifier: Set two below", "empty", "lb", lbFile, inBoth, emptySet("p:y.a.b.c.v", edit.Int(5)), edit.ReasonNone, ""},
	// E6 leaves the box out; its Undo puts it back where W7 puts it, after e.
	{"stated beside an identifier: Reset at it", "empty", "lb", lbFile, inBoth, wayOp(edit.OpReset, "p:y.a.b"), edit.ReasonNone,
		lineFile("p", "lb", "y", "a: { e: e1, b: { n: 2 }, k: 3 }")},
	{"stated beside an identifier: Set at it to its default", "empty", "lb", lbFile, inBoth, setTo("p:y.a.b", "{}"), edit.ReasonNone,
		lineFile("p", "lb", "y", "a: { e: e1, b: { n: 2 }, k: 3 }")},
	{"stated beside a load: Reset at it", "empty", "lc", lcbFile, inBoth, wayOp(edit.OpReset, "p:y.a.b"), edit.ReasonNone, ""},
	{"stated beside a load: Set below to its default", "empty", "lc", lcbFile, inBoth, emptySet("p:y.a.b.n", edit.Int(0)), edit.ReasonNone, ""},
	{"stated beside a load: Set at it", "empty", "lc", lcbFile, inBoth, setTo("p:y.a.b", "{ n: 7 }"), edit.ReasonNone, ""},
	// A default that reads a record parameter: the active Set to it leaves the field out (E6), the
	// inactive one is blind (DECISIONS 462); never taken for what a literal states.
	{"bound by a parameter, active: Set to its default", "empty", "lp", lpFile, inActive, emptySet("p:z.h.p.x", edit.Bool(true)), edit.ReasonNone, ""},
	{"bound by a parameter, active: Reset at it", "empty", "lp", lpFile, inActive, wayOp(edit.OpReset, "p:z.h.p"), edit.ReasonNone, ""},
	{"bound by a parameter, inactive: Set to its default", "empty", "lp", lpFile, inInactive, emptySet("p:z.h.p.x", edit.Bool(true)), edit.ReasonComputed, ""},
}

// API.md W7, W8, W11, W11a, E6, E22 (DECISIONS 273, 462): a record field whose default is `{}`
// is edited inside an edit layer's line like any other, Editable and Apply alike, the layer active
// or not, and its Undo gives the bytes back.
func TestLayerParityEmptyDefault(t *testing.T) { runParity(t, emptyLaw(), emptyRows) }
