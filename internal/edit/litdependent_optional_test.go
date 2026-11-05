package edit_test

import (
	"context"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// optSrc's field pick has a type whose first branch, `[Int]?`, is an optional list; its entry
// holds none, so no literal of the package states the branch but the edit below.
const optSrc = `package d

local enum Kind { a, b }

local record Ev {
  kind: Kind
}

local let evs: table Ev = { one { kind: a }, two { kind: b } }

local type Pick(e: Ev) = match e.kind {
  a => [Int]?
  b => [String]
}

local record Held {
  ev: ref evs?
  pick: Pick(ev)
}

local let held: Held = { ev: one, pick: none }
`

// API.md V1, DECISIONS 171: an uncomputed dependent type takes a Source list in its first list branch, an optional one too;
// its argument names no entry, since one reading none computes Never (DECISIONS 307).
func TestDependentSourceTakesOptionalListBranch(t *testing.T) {
	s := open(t, mapFS{"law/project.canon": file(projectCanon), "law/d/d.canon": file(optSrc)}, nil, "", "d")
	if n := s.a.Result().Summary.Errors; n > 0 {
		t.Fatalf("optSrc has %d errors: %v", n, s.a.Result().List)
	}
	ops := []edit.Operation{setOf("d:held", edit.Source(`{ ev: nope, pick: [2] }`))}
	if _, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: ops}); err != nil {
		t.Errorf("a list given to the optional list branch: %v, want it applied", err)
	}
}
