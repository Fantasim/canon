package edit_test

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/wire"
)

// judgeLaw is a variant whose case b judges the fields a keeps: x, y and v by their
// refinements, z identical in both, item required; c keeps z alone.
const judgeLaw = `package ev

/// What an event does.
variant Kind {
  /// The old case.
  a {
    /// A count.
    x: Int(1..=10) = 1
    /// A name.
    y: String = ""
    /// A list.
    v: [Int] = []
    /// A note.
    z: Int(0..=9) = 0
  }
  /// The new case.
  b {
    /// The item.
    item: String
    /// A count.
    x: Int(1..=5) = 1
    /// A name.
    y: String(..=3) = ""
    /// A list.
    v: [Int where it > 0](..=5) = []
    /// A note.
    z: Int(0..=9) = 0
  }
  /// A case keeping z alone, of an identical type.
  c {
    /// The item.
    item: String
    /// A note.
    z: Int(0..=9) = 0
  }
}

/// An event.
record Event {
  /// What it does.
  kind: Kind
}

/// The events.
let events: table Event = {
  e { kind: a { %VALUE% } }
}
`

// judgeFS is judgeLaw with e's old case written as value.
func judgeFS(value string) mapFS {
	return mapFS{
		"law/project.canon": file("project acme {\n  canon: \"0.1\"\n}\n"),
		"law/ev/ev.canon":   file(strings.Replace(judgeLaw, "%VALUE%", value, 1)),
	}
}

// applyOps applies ops, as an edit's JSON form gives them, to fsys's package ev; hosts counts
// the analyses Apply asks a host for.
func applyOps(t *testing.T, fsys mapFS, ops string, hosts *int) *edit.Plan {
	t.Helper()
	var req struct {
		Ops []edit.Operation `json:"ops"`
	}
	if err := json.Unmarshal([]byte(`{"ops": [`+ops+`]}`), &req); err != nil {
		t.Fatal(err)
	}
	s := open(t, fsys, nil, "", "ev")
	s.env.Host = func(a *build.Analysis) wire.Host {
		if hosts != nil {
			*hosts++
		}
		return hostOf(a)
	}
	plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: req.Ops})
	if err != nil {
		t.Fatal(err)
	}
	return plan
}

func droppedPaths(plan *edit.Plan) []string {
	var out []string
	for _, d := range plan.Dropped {
		out = append(out, d.Path)
	}
	return out
}

const setCaseB = `{"op": "setCase", "path": "events.e.kind", "case": "b", "source": "{ item: \"I\" }"}`

// API.md E14 (log-2026-09-29 M4 B3-r): kept fields are judged together; only the ones whose
// value breaks the new type are Dropped, the others kept, whatever their number.
func TestSetCaseDropsOnlyTheBadFields(t *testing.T) {
	plan := applyOps(t, judgeFS(`x: 2, y: "abcd", v: [1, 2], z: 3`), setCaseB, nil)
	if got, want := droppedPaths(plan), []string{"ev:events.e.kind.y"}; !slices.Equal(got, want) {
		t.Errorf("Dropped %v, want %v", got, want)
	}
	after := string(plan.Changes[0].After)
	if strings.Contains(after, `"abcd"`) || !strings.Contains(after, "x: 2") || !strings.Contains(after, "v: [1, 2]") || !strings.Contains(after, "z: 3") {
		t.Errorf("y is kept, or x, v or z is not:\n%s", after)
	}
}

// API.md E14, E19 (log-2026-09-29 M4 B3-r): a kept field is judged even when the new case
// value cannot be read, here for its missing required item: x's 7 breaks Int(1..=5) and is
// Dropped, so a draft (AllowErrors) never writes it; z fits and is kept.
func TestSetCaseJudgesAnUnreadableVariant(t *testing.T) {
	plan := applyOps(t, judgeFS(`x: 7, z: 3`), `{"op": "setCase", "path": "events.e.kind", "case": "b"}`, nil)
	if got, want := droppedPaths(plan), []string{"ev:events.e.kind.x"}; !slices.Equal(got, want) {
		t.Errorf("Dropped %v, want %v", got, want)
	}
	if after := string(plan.Changes[0].After); !strings.Contains(after, `b { z: 3 }`) {
		t.Errorf("x is kept or z is not:\n%s", after)
	}
}

// API.md E1, E14 (log-2026-09-29 M4 B3-r): a SetCase after another operation judges the state
// that operation left, as the SetCase alone on that state does.
func TestSetCaseAfterAnOperationEqualsAlone(t *testing.T) {
	jsonFS := mapFS{
		"law/project.canon":  file("project acme {\n  canon: \"0.1\"\n}\n"),
		"law/ev/ev.canon":    file(setCaseJSON),
		"law/ev/events.json": file("[\n  {\n    \"id\": \"e\",\n    \"type\": \"a\",\n    \"ps\": [\n      {\n        \"n\": 1\n      }\n    ]\n  }\n]\n"),
	}
	for _, tc := range []struct {
		fsys         mapFS
		set, setCase string
		dropped      string
	}{
		{judgeFS(`v: [1]`), `{"op": "set", "path": "events.e.kind.v", "source": "[-1]"}`, setCaseB, "ev:events.e.kind.v"},
		{jsonFS, `{"op": "set", "path": "events[e].kind.ps[0].n", "value": 9}`, `{"op": "setCase", "path": "events[e].kind", "case": "b"}`, "ev:events[e].kind.ps"},
	} {
		both := applyOps(t, tc.fsys, tc.set+", "+tc.setCase, nil)
		alone := applyOps(t, written(tc.fsys, applyOps(t, tc.fsys, tc.set, nil)), tc.setCase, nil)
		if !slices.Equal(droppedPaths(both), droppedPaths(alone)) || !slices.Contains(droppedPaths(both), tc.dropped) {
			t.Errorf("Dropped %v after a Set, %v alone; want %s in both", droppedPaths(both), droppedPaths(alone), tc.dropped)
		}
		if string(both.Changes[0].After) != string(alone.Changes[0].After) {
			t.Errorf("after a Set:\n%s\nalone:\n%s", both.Changes[0].After, alone.Changes[0].After)
		}
	}
}

// API.md E14 (log-2026-09-29 M4 B3-r), NFR-01: a SetCase whose kept fields have identical
// types asks for no analysis past the base's; one keeping a field whose refinements differ
// asks for exactly one more, to judge it.
func TestSetCaseAnalysesOnlyToJudge(t *testing.T) {
	for _, tc := range []struct {
		value, to string
		want      int
	}{{`z: 3`, "c", 1}, {`x: 2, z: 3`, "b", 2}, {`x: 7, z: 3`, "b", 2}} {
		hosts := 0
		op := `{"op": "setCase", "path": "events.e.kind", "case": "` + tc.to + `", "source": "{ item: \"I\" }"}`
		applyOps(t, judgeFS(tc.value), op, &hosts)
		if hosts != tc.want {
			t.Errorf("a { %s } to %s: %d analyses, want %d", tc.value, tc.to, hosts, tc.want)
		}
	}
}

// defaultLaw is a variant left at its default, one and two levels below a table entry, and as
// an element of a defaulted list and a value of a defaulted map.
const defaultLaw = `package ev

/// What an event does.
variant Kind {
  /// The old case.
  a {
    /// A count.
    x: Int(1..=10) = 1
  }
  /// The new case.
  b {
    /// A count.
    x: Int(1..=5) = 1
  }
}

/// A holder.
record Holder {
  /// What it holds.
  kind: Kind = a { x: 8 }
  /// A list it holds.
  kinds: [Kind] = [a { x: 8 }]
  /// A map it holds.
  byName: {String: Kind} = { "k": a { x: 8 } }
}

/// An event.
record Event {
  /// What it does.
  kind: Kind = a { x: 8 }
  /// Its holder.
  holder: Holder = {}
}

/// The events.
let events: table Event = {
  e {}
}
`

// API.md E14, W7, W8 (log-2026-09-29 M4 B3-r2): a variant left at its default is judged too,
// its literal located below the nearest one the source writes: x's 8 breaks Int(1..=5).
func TestSetCaseJudgesADefaultVariant(t *testing.T) {
	fsys := mapFS{"law/project.canon": file("project acme {\n  canon: \"0.1\"\n}\n"), "law/ev/ev.canon": file(defaultLaw)}
	for _, path := range []string{"events.e.kind", "events.e.holder.kind"} {
		plan := applyOps(t, fsys, `{"op": "setCase", "path": "`+path+`", "case": "b"}`, nil)
		if got, want := droppedPaths(plan), []string{"ev:" + path + ".x"}; !slices.Equal(got, want) {
			t.Errorf("%s: Dropped %v, want %v", path, got, want)
		}
		after := string(plan.Changes[0].After)
		if data := after[strings.Index(after, "let events"):]; strings.Contains(data, "x: 8") || !strings.Contains(data, "kind: b") {
			t.Errorf("%s: x is kept or b not written:\n%s", path, data)
		}
		checkUndo(t, goldenCase{fsys: fsys, pkgs: []string{"ev"}}, nil, plan)
	}
}

// API.md W3, W5, W8, E14 (log-2026-09-29 M4 B3-r2): W8 materializes only a defaulted record's
// fields, so an element of a defaulted list or a value of a defaulted map has no structural
// path: its SetCase is refused, reason computed, before any kept field needs judging.
func TestSetCaseRefusesAVariantInsideADefaultedCollection(t *testing.T) {
	fsys := mapFS{"law/project.canon": file("project acme {\n  canon: \"0.1\"\n}\n"), "law/ev/ev.canon": file(defaultLaw)}
	for _, path := range []string{"events.e.holder.kinds[0]", "events.e.holder.byName[k]"} {
		var req struct {
			Ops []edit.Operation `json:"ops"`
		}
		if err := json.Unmarshal([]byte(`{"ops": [{"op": "setCase", "path": "`+path+`", "case": "b"}]}`), &req); err != nil {
			t.Fatal(err)
		}
		s := open(t, fsys, nil, "", "ev")
		_, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: req.Ops})
		var ne *edit.NotEditableError
		if !errors.As(err, &ne) || ne.Reason != edit.ReasonComputed {
			t.Errorf("%s: %v, want not editable, reason computed", path, err)
		}
	}
}
