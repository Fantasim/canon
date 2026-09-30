package edit_test

import (
	"bytes"
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// projectCanon is the project file of the forged writes' project.
const projectCanon = "project acme {\n  canon: \"0.1\"\n}\n"

// regionSrc is a file for the forged writes of TestRegionCheckRejectsForgedWrites.
const regionSrc = `package p

/// A row.
record Row {
  /// A.
  a: Int
}

/// Rows.
let rows: table Row = {
  one { a: 1 }
  two { a: 2 }
  three { a: 3 }
  four { a: 4 }
}

/// Other.
let other: Int = 5

/// More.
let more: Int = 6

/// Columns.
record Cols {
  /// A.
  a: Int = 0
  /// B.
  b: Int = 0
  /// C.
  c: Int = 0
  /// D.
  d: Int = 0
}

/// One row of columns.
let row: Cols = {
  a: 1
  b: 2
  d: 4
}

/// Notes.
let notes: table Row = {
  // the first note
  /// First.
  n1 { a: 7 } // seven
  n2 { a: 8 }
}

/// Loaded rows.
let loaded: table Row = load("rows.json")
`

// regionJSON is the JSON source of regionSrc's loaded rows.
const regionJSON = `{
  "x": {
    "a": 1
  },
  "y": {
    "a": 2
  },
  "z": {
    "a": 3
  }
}
`

// moveNote moves regionSrc's commented note n1 last (log-2026-09-29 M4 U1b).
var moveNote = edit.Operation{Kind: edit.OpMove, Path: "notes.n1", Index: 1}

// API.md M4, M6, E23: a Move's regions are the moved item's lines, its comments with them, gone,
// and at the insertion point after the last note, exactly those lines; its inverse gives the
// bytes back (log-2026-09-29 M4 U1b).
func TestMoveRegions(t *testing.T) {
	fsys := mapFS{"law/project.canon": file(projectCanon), "law/p/p.canon": file(regionSrc), "law/p/rows.json": file(regionJSON)}
	s := open(t, fsys, nil, "", "p")
	plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{moveNote}})
	if err != nil {
		t.Fatal(err)
	}
	w := edit.Writes(plan, "p/p.canon")[0]
	lines := "  // the first note\n  /// First.\n  n1 { a: 7 } // seven\n"
	lo := strings.Index(regionSrc, lines)
	at := strings.Index(regionSrc, "  n2 { a: 8 }\n") + len("  n2 { a: 8 }\n")
	endOf := func(item string) int { return strings.Index(regionSrc, item) + len(item) }
	want := []edit.Region{
		{Lo: lo, Hi: lo + len(lines), Kind: edit.RegionGone},
		{Lo: at, Hi: at, Kind: edit.RegionMoved, FromLo: lo, FromHi: lo + len(lines), Comma: endOf("n1 { a: 7 }")},
		{Lo: endOf("n2 { a: 8 }"), Hi: endOf("n2 { a: 8 }"), Kind: edit.RegionComma},
	}
	if !slices.Equal(w.Regions, want) {
		t.Errorf("regions %+v, want %+v", w.Regions, want)
	}
	if moved := strings.Replace(regionSrc, lines, "", 1); string(w.After) != strings.Replace(moved, "  n2 { a: 8 }\n", "  n2 { a: 8 }\n"+lines, 1) {
		t.Errorf("the note did not move with its comments:\n%s", w.After)
	}
	back := open(t, written(fsys, plan), nil, "", "p")
	undo, err := edit.Apply(context.Background(), back.env, back.snap, edit.Request{Ops: plan.Undo})
	if err != nil || string(written(fsys, undo)["law/p/p.canon"].Data) != regionSrc {
		t.Errorf("API.md E23: the Move's inverse does not give the file back, comments included: %v", err)
	}
}

// keywordMap is a map whose key "in" makes DECISIONS 211 keep a comma after the item before it.
const keywordMap = "package p\n\n/// E.\nenum E { a, in, b }\n\n/// W.\nlet w: {E: Int} = {\n  a: 1\n  b: 2,\n  in: 3\n}\n"

// docComma is a table whose first entry keeps a comma on its own line so that `/// d1` documents
// nothing after it (DECISIONS 216).
const docComma = "package p\n\n/// Row.\nrecord Row {\n  /// A.\n  a: Int\n}\n\n/// Rows.\nlet rows: table Row = {\n  one { a: 1 }\n  /// d1\n  ,\n  two { a: 2 }\n}\n"

// docCommaThree is docComma with a third entry, so the comma still keeps `/// d1` off it when two goes.
var docCommaThree = strings.Replace(docComma, "  two { a: 2 }\n", "  two { a: 2 }\n  three { a: 3 }\n", 1)

// docCommaNote is docComma whose comma carries a comment, which stays when the settle drops it.
var docCommaNote = strings.Replace(docComma, "  ,\n", "  , // c\n", 1)

// API.md M5, M6 (log-2026-09-29 M4 U1b-r, G2 refined): the comma a kept neighbour gains or loses
// when the item after it changes, a 216 comma on its own line included, is a re-print M5
// requires, which the check accepts for a Move, a Remove and an Insert alike.
func TestNeighbourCommaRegions(t *testing.T) {
	withoutIn := strings.Replace(keywordMap, "  b: 2,\n  in: 3\n", "  b: 2\n", 1)
	for _, c := range []struct {
		name, src string
		op        edit.Operation
		want      string
	}{
		{"move in first", keywordMap, edit.Operation{Kind: edit.OpMove, Path: "w[in]", Index: 0}, "  in: 3\n  a: 1\n  b: 2\n}"},
		{"remove in", keywordMap, edit.Operation{Kind: edit.OpRemove, Path: "w[in]"}, "  a: 1\n  b: 2\n}"},
		{"add in", withoutIn, edit.Operation{Kind: edit.OpAddEntry, Path: "w", Key: edit.Member("in"), Value: edit.Int(4)}, "  a: 1\n  b: 2,\n  in: 4\n}"},
		{"remove after a 216 comma line", docComma, edit.Operation{Kind: edit.OpRemove, Path: "rows.two"}, "  one { a: 1 }\n  /// d1\n}"},
		{"remove after a kept 216 comma line", docCommaThree, edit.Operation{Kind: edit.OpRemove, Path: "rows.two"}, "  one { a: 1 }\n  /// d1\n  ,\n  three { a: 3 }\n}"},
		{"remove after a 216 comma line with a comment", docCommaNote, edit.Operation{Kind: edit.OpRemove, Path: "rows.two"}, "  one { a: 1 }\n  /// d1\n  // c\n}"},
	} {
		fsys := mapFS{"law/project.canon": file(projectCanon), "law/p/p.canon": file(c.src)}
		s := open(t, fsys, nil, "", "p")
		plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{c.op}})
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		w := edit.Writes(plan, "p/p.canon")[0]
		if !strings.HasSuffix(string(w.After), c.want+"\n") || !keptOutside(t, "p/p.canon", w) {
			t.Errorf("%s: want a write ending %q the check accepts, got regions %+v:\n%s", c.name, c.want, w.Regions, w.After)
		}
	}
}

// API.md M6 (log-2026-09-29 M4 U4b-r3, U1b, U1b-r): Apply's own writes pass, each forged one
// fails: a sibling changed beside a removal, Set or move, a moved item or its comments changed,
// items more than inserted, blank lines moved or added, in .canon and in JSON.
func TestRegionCheckRejectsForgedWrites(t *testing.T) {
	for _, c := range []struct {
		name, file string
		op         edit.Operation
		old, new   string
	}{
		{"sibling", "p/p.canon", edit.Operation{Kind: edit.OpRemove, Path: "rows.two"}, "four { a: 4 }", "four { a: 44 }"},
		{"insertion", "p/p.canon", edit.Operation{Kind: edit.OpSet, Path: "other", Value: edit.Int(9)}, "= 9\n", "= 9\nlet junk: Int = 0\n"},
		{"blank dropped", "p/p.canon", edit.Operation{Kind: edit.OpSet, Path: "other", Value: edit.Int(9)}, "}\n\n/// Rows.", "}\n/// Rows."},
		{"blank added", "p/p.canon", edit.Operation{Kind: edit.OpSet, Path: "other", Value: edit.Int(9)}, "package p\n", "package p\n\n"},
		{"blank beside a removal", "p/p.canon", edit.Operation{Kind: edit.OpRemove, Path: "rows.four"}, "}\n\n/// Other.", "}\n/// Other."},
		{"sibling of a move", "p/p.canon", edit.Operation{Kind: edit.OpMove, Path: "rows.one", Index: 1}, "four { a: 4 }", "four { a: 44 }"},
		{"sibling beside a move", "p/p.canon", moveNote, "n2 { a: 8 }", "n2 { a: 88 }"},
		{"moved item changed", "p/p.canon", edit.Operation{Kind: edit.OpMove, Path: "rows.one", Index: 1}, "one { a: 1 }", "one { a: 11 }"},
		{"moved comment dropped", "p/p.canon", moveNote, "  // the first note\n", ""},
		{"moved doc changed", "p/p.canon", moveNote, "/// First.", "/// Last."},
		{"moved trailing comment changed", "p/p.canon", moveNote, "// seven", "// eight"},
		{"comma in a moved comment", "p/p.canon", moveNote, "// seven", "//, seven"},
		{"comma in a moved doc", "p/p.canon", moveNote, "/// First.", "/// Fi,rst."},
		{"blank line arriving with a move", "p/p.canon", moveNote, "  n2 { a: 8 }\n  // the first note", "  n2 { a: 8 }\n\n  // the first note"},
		{"comma inside a neighbour of a move", "p/p.canon", moveNote, "n2 { a: 8 }", "n2 { a: 8, }"},
		{"sibling of a field swap", "p/p.canon", edit.Operation{Kind: edit.OpSet, Path: "row", Value: edit.Source("{ a: 1, c: 3, d: 4 }")}, "d: 4", "d: 5"},
		{"item more than inserted", "p/p.canon", edit.Operation{Kind: edit.OpAddEntry, Path: "rows", Key: edit.Key("five"), Value: edit.Source("{ a: 5 }")}, "  five { a: 5 }\n", "  five { a: 5 }\n  six { a: 6 }\n"},
		{"JSON sibling of a move", "p/rows.json", edit.Operation{Kind: edit.OpMove, Path: "loaded.x", Index: 1}, `"a": 3`, `"a": 33`},
		{"JSON item more than inserted", "p/rows.json", edit.Operation{Kind: edit.OpAddEntry, Path: "loaded", Key: edit.Key("w"), Value: edit.Source("{ a: 9 }")}, `"a": 9
  }`, `"a": 9
  },
  "v": {
    "a": 8
  }`},
		{"JSON sibling of a Set", "p/rows.json", edit.Operation{Kind: edit.OpSet, Path: "loaded.y.a", Value: edit.Int(5)}, `"a": 3`, `"a": 33`},
		{"JSON sibling before a removal", "p/rows.json", edit.Operation{Kind: edit.OpRemove, Path: "loaded.y"}, `"a": 1`, `"a": 11`},
		{"JSON sibling after a removal", "p/rows.json", edit.Operation{Kind: edit.OpRemove, Path: "loaded.y"}, `"a": 3`, `"a": 33`},
		{"JSON member added beside a Set", "p/rows.json", edit.Operation{Kind: edit.OpSet, Path: "loaded.y.a", Value: edit.Int(5)}, `"a": 5`, `"a": 5, "b": 0`},
	} {
		fsys := mapFS{"law/project.canon": file(projectCanon), "law/p/p.canon": file(regionSrc), "law/p/rows.json": file(regionJSON)}
		s := open(t, fsys, nil, "", "p")
		plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{c.op}})
		if err != nil {
			t.Fatal(err)
		}
		w := edit.Writes(plan, c.file)[0]
		if !keptOutside(t, c.file, w) {
			t.Errorf("%s: Apply's own write fails the check", c.name)
		}
		forged := w
		forged.After = bytes.Replace(w.After, []byte(c.old), []byte(c.new), 1)
		switch {
		case bytes.Equal(forged.After, w.After):
			t.Errorf("%s: the forgery changes nothing", c.name)
		case keptOutside(t, c.file, forged):
			t.Errorf("%s: a forged write passes the check (API.md M6):\n%s", c.name, forged.After)
		}
	}
}
