//go:build knownbug

package edit_test

import (
	"context"
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

const setCaseRangeLaw = `package ev

/// What an event does.
variant Kind {
  /// Spawns in a region.
  spawn {
    /// The region.
    region: Int
    /// How many.
    count: Int(1..=10) = 1
  }
  /// Drops an item.
  drop {
    /// The item.
    item: String
    /// How many.
    count: Int(1..=3) = 1
  }
}

/// An event.
record Event {
  /// What it does.
  kind: Kind
}

/// The events.
let events: table Event = {
  rain { kind: spawn { region: 1, count: 4 } }
}
`

// KNOWN BUG (run with -tags knownbug), API.md E14: a field kept by SetCase must satisfy the new
// field's refinements; `count: 4` does not satisfy `Int(1..=3)`, so it must be Dropped and left
// at the default. Apply keeps it, the re-check poisons the value, and the Undo cannot apply.
func TestSetCaseDropsRangeViolation(t *testing.T) {
	fsys := mapFS{"law/project.canon": file("project acme {\n  canon: \"0.1\"\n}\n"), "law/ev/ev.canon": file(setCaseRangeLaw)}
	var req struct {
		Ops []edit.Operation `json:"ops"`
	}
	src := `{"ops": [{"op": "setCase", "path": "events.rain.kind", "case": "drop", "source": "{ item: \"II_MOON\" }"}]}`
	if err := json.Unmarshal([]byte(src), &req); err != nil {
		t.Fatal(err)
	}
	s := open(t, fsys, nil, "", "ev")
	plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: req.Ops})
	if err != nil {
		t.Fatal(err)
	}
	dropped := slices.IndexFunc(plan.Dropped, func(d edit.Dropped) bool { return strings.HasSuffix(d.Path, "kind.count") })
	if dropped < 0 {
		t.Errorf("count: 4 does not fit Int(1..=3) and is not in Dropped: %+v", plan.Dropped)
	}
	if got := string(plan.Changes[0].After); strings.Contains(got, "count: 4") {
		t.Errorf("the out-of-range count is kept:\n%s", got)
	}
}
