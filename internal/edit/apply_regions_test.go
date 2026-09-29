package edit_test

import (
	"bytes"
	"context"
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

// API.md M6 (log-2026-09-29 M4 U4b-r3): the check passes Apply's own writes and fails each
// forged one: a sibling changed beside a removal, Set or move, an item or declaration more
// than the plan inserts, blank lines moved away from the regions, in .canon and in JSON.
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
