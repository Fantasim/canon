package edit_test

import (
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// boundTypes are a record whose field defaults to its parameter, and a holder whose p defaults to a
// spread of an earlier field, so the field p's default gives is not the one `{}` leaves it to.
const boundTypes = `/// Bound by m.
record P(m: Bool) {
  /// X.
  x: Bool = m
}

/// H.
record H {
  /// N.
  n: Int = 0
  /// A flag.
  flag: Bool = true
  /// The other.
  p2: P(flag) = { x: false }
  /// Bound, spread from the other.
  p: P(flag) = { ...p2 }
}

/// Z.
record Z {
  /// H.
  h: H = {}
}

/// A holder whose bound field defaults to {}.
record PH {
  /// A flag.
  flag: Bool = true
  /// Bound.
  p: P(flag) = {}
}
`

// e6RecSrc holds a record field whose default is `{}` and boundTypes' holders, each written in
// Canon and in JSON.
const e6RecSrc = `package d

/// A cell.
record S {
  /// A.
  a: Int = 1
  /// B.
  b: Int = 2
}

/// A holder.
record R {
  /// Its cell.
  s: S = {}
  /// N.
  n: Int = 0
}

` + boundTypes + `
/// In Canon.
let r: R = { s: { a: 5 }, n: 1 }

/// In JSON.
let j: R = load("j.json")

/// Bound, in Canon.
let bz: Z = { h: { p: { x: true } } }

/// Bound, in JSON.
let jz: Z = load("z.json")

/// Bound to {}, in Canon.
let ph: PH = { p: { x: false } }

/// Bound to {}, in JSON.
let jph: PH = load("ph.json")
`

const (
	e6RecFile = "law/d/d.canon"
	e6RecJSON = "{\n  \"s\": {\n    \"a\": 5\n  },\n  \"n\": 1\n}\n"
	e6BzJSON  = "{\n  \"h\": {\n    \"p\": {\n      \"x\": true\n    }\n  }\n}\n"
	e6PhJSON  = "{\n  \"p\": {\n    \"x\": false\n  }\n}\n"
)

func e6RecFS() mapFS {
	return mapFS{"law/project.canon": file(projectCanon), e6RecFile: file(e6RecSrc), "law/d/j.json": file(e6RecJSON),
		"law/d/z.json": file(e6BzJSON), "law/d/ph.json": file(e6PhJSON)}
}

// API.md E6, TYPES.md 7.5, 11.1, M8 (EDIT-E6RECORD): a base Set to a value equal to the field's
// default, fields left out read as their defaults with their parameters bound, removes it in Canon
// and JSON alike; a value that differs is written. The Undo restores every value (E22).
func TestE6RecordDefaultBase(t *testing.T) {
	for _, c := range []struct {
		name, path, src, file, want string
	}{
		{"canon, empty", "r.s", "{}", e6RecFile, "let r: R = { n: 1 }\n"},
		{"canon, partly written", "r.s", "{ a: 1 }", e6RecFile, "let r: R = { n: 1 }\n"},
		{"canon, fully written", "r.s", "{ a: 1, b: 2 }", e6RecFile, "let r: R = { n: 1 }\n"},
		{"canon, below a whole Set", "r", "{ s: { b: 2 }, n: 1 }", e6RecFile, "let r: R = { n: 1 }\n"},
		{"json, empty", "j.s", "{}", "law/d/j.json", "{\n  \"n\": 1\n}\n"},
		{"json, partly written", "j.s", "{ b: 2 }", "law/d/j.json", "{\n  \"n\": 1\n}\n"},
		{"json, below a whole Set", "j", "{ s: { a: 1 }, n: 1 }", "law/d/j.json", "{\n  \"n\": 1\n}\n"},
		{"canon, unlike a spread default", "bz.h.p", "{}", e6RecFile, "let bz: Z = { h: { p: {} } }\n"},
		{"json, unlike a spread default", "jz.h.p", "{}", "law/d/z.json", "{\n  \"h\": {\n    \"p\": {}\n  }\n}\n"},
		{"canon, a bound default", "ph.p", "{}", e6RecFile, "let ph: PH = {}\n"},
		{"json, a bound default", "jph.p", "{}", "law/d/ph.json", "{}\n"},
		{"canon, a bound default, written", "ph.p", "{ x: true }", e6RecFile, "let ph: PH = {}\n"},
	} {
		e6Base(t, c.name, setTo(c.path, c.src), c.file, c.want)
	}
}

// e6Base applies op, checks that file then holds want, and undoes it twice (E22).
func e6Base(t *testing.T, name string, op edit.Operation, file, want string) {
	t.Helper()
	edited, ok := applied(t, name, e6RecFS(), []edit.Operation{op})
	if !ok {
		return
	}
	if got := string(edited.fsys[file].Data); !strings.Contains(got, want) {
		t.Errorf("%s: %s is\n%s\nwant it to hold\n%s", name, file, got, want)
	}
	undoTwice(t, name, e6RecFS(), []edit.Operation{op})
}

// boundSrc is boundTypes in package d, its holder left to its default; boundH2 is it with p's
// default a value no spread gives.
const boundSrc = "package d\n\n" + boundTypes + "\n/// A bound holder.\nlet z: Z = {}\n"

const sbFile = "law/d/sb.layer.canon"

var boundH2 = strings.Replace(boundSrc, "p: P(flag) = { ...p2 }", "p: P(flag) = { x: false }", 1)

// boundLaw is src with layer sb's one all-literal line amending z.
func boundLaw(src, line string) mapFS {
	return mapFS{"law/project.canon": file(projectCanon), "law/d/d.canon": file(src), sbFile: file(lineFile("d", "sb", "z", line))}
}

// boundLaws are the layer laws: p at the value `{}` gives it (same), at another (other), the same
// with H2's default (given), and lines holding `{}` for a bound record (empty p, empty p2, H2).
func boundLaws() map[string]mapFS {
	return map[string]mapFS{
		"same":    boundLaw(boundSrc, "h: { n: 1, p: { x: true } }"),
		"other":   boundLaw(boundSrc, "h: { n: 1, p: { x: false } }"),
		"given":   boundLaw(boundH2, "h: { n: 1, p: { x: true } }"),
		"emptyP":  boundLaw(boundSrc, "h: { n: 1, p: {} }"),
		"emptyP2": boundLaw(boundSrc, "h: { n: 1, p2: {} }"),
		"h2":      boundLaw(boundH2, "h: { p: {} }"),
	}
}

// boundOps are the requests each layer law takes: Resets, Sets at each depth, two-op requests.
var boundOps = [][]edit.Operation{
	{{Kind: edit.OpReset, Path: "d:z.h"}},
	{{Kind: edit.OpReset, Path: "d:z.h.p"}},
	{emptySet("d:z.h.p.x", edit.Bool(false))},
	{emptySet("d:z.h.p.x", edit.Bool(true))},
	{setTo("d:z.h", "{ n: 3 }")},
	{setTo("d:z.h", "{ n: 2, p: {} }")},
	{setTo("d:z.h.p", "{}")},
	{setTo("d:z.h.p", "{ x: false }")},
	{setTo("d:z.h.p2", "{}")},
	{emptySet("d:z.h.p.x", edit.Bool(false)), emptySet("d:z.h.n", edit.Int(4))},
	{emptySet("d:z.h.flag", edit.Bool(false)), setTo("d:z.h.p", "{}")},
}

// API.md E6, E22, W11, TYPES.md 11.1 (EDIT-E6RECORD round 2): in an all-literal layer line, a
// bound record's defaults are computed with its parameters bound, so every edit applies, active
// layer, and its Undo restores every value with the layer active and not (E22: values, not text).
func TestBoundDefaultLayerLine(t *testing.T) {
	laws := boundLaws()
	for _, law := range []string{"same", "other", "given", "emptyP", "emptyP2", "h2"} {
		for _, ops := range boundOps {
			undoIn(t, undoSpec{name: law + ": " + opsText(ops), fsys: laws[law], view: undoView{[]string{"sb"}, "sb"},
				ops: ops, views: [][]string{{"sb"}, nil}})
		}
	}
}
