package edit_test

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// setCaseLaw is a variant whose two cases both have a field v: old in case a, nw in case b.
const setCaseLaw = `package ev

/// What an event does.
variant Kind {
  /// The old case.
  a {
    /// The value.
    v: %OLD%
  }
  /// The new case.
  b {
    /// The item.
    item: String = ""
    /// The value.
    v: %NEW%
  }
}

/// An event.
record Event {
  /// What it does.
  kind: Kind
}

/// The events.
let events: table Event = {
  e { kind: a { v: %VALUE% } }
}
`

// setCaseRow is a SetCase from a { v: value } to b, whose v differs from a's by refinements
// alone: v is kept when value satisfies b's refinements, Dropped otherwise.
type setCaseRow struct {
	name, old, nw, value string
	dropped              bool
}

// Every refinement kind of TYPES.md 7.4, and the refinements inside a field's type (a list's
// element, a map's value, an optional's value).
var setCaseRows = []setCaseRow{
	{"range kept", "Int(1..=10) = 1", "Int(1..=3) = 1", "2", false},
	{"range", "Int(1..=10) = 1", "Int(1..=3) = 1", "4", true},
	{"float range", "Float = 0.5", "Float(0.0..=1.0) = 0.5", "2.5", true},
	{"duration range", "Duration = 1s", "Duration(..=10s) = 1s", "1m", true},
	{"string length", `String = ""`, `String(..=3) = ""`, `"abcd"`, true},
	{"list length", "[Int] = []", "[Int](..=2) = []", "[1, 2, 3]", true},
	{"map length", "{String: Int} = {}", "{String: Int}(..=1) = {}", `{ "a": 1, "b": 2 }`, true},
	{"pattern kept", `String = "a"`, `String(/^[a-z]+$/) = "a"`, `"abc"`, false},
	{"pattern", `String = "a"`, `String(/^[a-z]+$/) = "a"`, `"A1"`, true},
	{"where kept", "Int = 3", "Int where it > 2 = 3", "4", false},
	{"where", "Int = 3", "Int where it > 2 = 3", "1", true},
	{"asset kept", `String = "a.png"`, `asset("icons", ext: [png]) = "a.png"`, `"b.png"`, false},
	{"asset missing", `String = "a.png"`, `asset("icons", ext: [png]) = "a.png"`, `"z.png"`, true},
	{"asset extension", `String = "a.png"`, `asset("icons", ext: [png]) = "a.png"`, `"a.jpg"`, true},
	{"list element", "[Int] = []", "[Int(1..=3)] = []", "[1, 5]", true},
	{"map value", "{String: Int} = {}", "{String: Int(..=3)} = {}", `{ "a": 5 }`, true},
	{"optional", "Int? = none", "Int(1..=3)? = none", "4", true},
}

// API.md E14: SetCase keeps an old field whose type is the new one's, refinements aside, only
// when its value satisfies the new field's refinements, whatever their kind (TYPES.md 7.4); a
// field not kept is Dropped and left to its default, and the Undo applies (E22).
func TestSetCaseDropsRangeViolation(t *testing.T) {
	for _, row := range setCaseRows {
		t.Run(row.name, func(t *testing.T) {
			law := strings.NewReplacer("%OLD%", row.old, "%NEW%", row.nw, "%VALUE%", row.value).Replace(setCaseLaw)
			fsys := mapFS{"law/project.canon": file("project acme {\n  canon: \"0.1\"\n}\n"), "law/ev/ev.canon": file(law), "law/ev/icons/a.png": file(""), "law/ev/icons/b.png": file("")}
			plan := applySetCase(t, fsys, "events.e.kind")
			dropped := slices.ContainsFunc(plan.Dropped, func(d edit.Dropped) bool { return d.Path == "ev:events.e.kind.v" })
			kept := strings.Contains(string(plan.Changes[0].After), "v: "+row.value)
			if dropped != row.dropped || kept == row.dropped {
				t.Errorf("v: %s from %s to %s: dropped %v, kept %v; want dropped %v (Dropped %+v)", row.value, row.old, row.nw, dropped, kept, row.dropped, plan.Dropped)
			}
			checkUndo(t, goldenCase{fsys: fsys, pkgs: []string{"ev"}}, nil, plan)
		})
	}
}

// setCaseJSON is TestSetCaseDropsRangeViolation's variant in a JSON source, where every
// refinement is judged at evaluation; ps's element already breaks P's own refinement.
const setCaseJSON = `package ev

/// What an event does.
variant Kind @json(tag: "type") {
  /// The old case.
  a {
    /// The value.
    v: Int(1..=10) = 1
    /// The parts.
    ps: [P] = []
    /// A name.
    s: String = "a"
    /// A note.
    w: String = ""
  }
  /// The new case.
  b {
    /// The value.
    v: Int(1..=3) = 1
    /// The parts.
    ps: [P](..=5) = []
    /// A name.
    s: String(/^[a-z]+$/) = "a"
    /// A note.
    w: String(..=5) = ""
  }
}

/// A part.
record P {
  /// Its count.
  n: Int(1..=3) = 1
}

/// An event.
record Event {
  /// Its id.
  id: String
  /// What it does.
  kind: Kind @json(inline)
}

/// The events.
let events: [Event] keyed by id = load("events.json")
`

// API.md E14 in a JSON source (log-2026-09-29 M4 B3-r): a kept field stays only if its value
// satisfies the new field type whole: v's range (E3204), s's pattern (E3205) and ps's element,
// which already broke P's refinement, drop them; w fits and is kept.
func TestSetCaseJSONDropsWholeTypeViolations(t *testing.T) {
	fsys := mapFS{
		"law/project.canon":  file("project acme {\n  canon: \"0.1\"\n}\n"),
		"law/ev/ev.canon":    file(setCaseJSON),
		"law/ev/events.json": file("[\n  {\n    \"id\": \"e\",\n    \"type\": \"a\",\n    \"v\": 4,\n    \"ps\": [\n      {\n        \"n\": 9\n      }\n    ],\n    \"s\": \"A1\",\n    \"w\": \"ok\"\n  }\n]\n"),
	}
	plan := applySetCase(t, fsys, "events[e].kind")
	var paths []string
	for _, d := range plan.Dropped {
		paths = append(paths, d.Path)
	}
	if want := []string{"ev:events[e].kind.ps", "ev:events[e].kind.s", "ev:events[e].kind.v"}; !slices.Equal(paths, want) {
		t.Errorf("Dropped %v, want %v", paths, want)
	}
	if after := string(plan.Changes[0].After); !strings.Contains(after, `"w": "ok"`) {
		t.Errorf("w is not kept:\n%s", after)
	}
	checkUndo(t, goldenCase{fsys: fsys, pkgs: []string{"ev"}}, nil, plan)
}

// applySetCase applies SetCase(path, b) to fsys's package ev.
func applySetCase(t *testing.T, fsys mapFS, path string) *edit.Plan {
	t.Helper()
	var req struct {
		Ops []edit.Operation `json:"ops"`
	}
	src := `{"ops": [{"op": "setCase", "path": "` + path + `", "case": "b"}]}`
	if err := json.Unmarshal([]byte(src), &req); err != nil {
		t.Fatal(err)
	}
	s := open(t, fsys, nil, "", "ev")
	plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: req.Ops})
	if err != nil {
		t.Fatal(err)
	}
	return plan
}
