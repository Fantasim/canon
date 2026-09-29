package edit_test

import (
	"context"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
)

// lockedLaw has a stable table, a plain table and an @codes enum.
const lockedLaw = `package p

/// A row.
record Row {
  /// Its label.
  label: String
}

/// An id.
record Id {
  /// Its label.
  label: String
  /// Its code, locked.
  code: Int @stable
}

/// Codes.
enum Code @codes(UInt8) { a = 1, b = 2 }

/// Ids that never change.
let ids: stable table Id = {
  one { label: "One", code: 1 }
  two { label: "Two", code: 2 }
}

/// Rows.
let rows: table Row = {
  x { label: "X" }
}
`

// API.md E20 (log-2026-09-29 M4 U5b-r, U5b-r2, U5b-r3): a plan names the ids its operations add to
// a stable table or retire, and those whose @stable field they set while the lock lacks them.
func TestPlanLocked(t *testing.T) {
	fsys := mapFS{
		"law/project.canon": file("project acme {\n  canon: \"0.1\"\n}\n"), "law/p/p.canon": file(lockedLaw),
		"law/p/canon.lock": file("# canon.lock v1\nfield  p.ids.code  1  one\ntable  p.ids  one\n"), // two: added by hand since the last build
	}
	p, err := build.Open(fsys, "/law", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	a, err := p.Analyze(context.Background(), []string{"p"})
	if err != nil {
		t.Fatal(err)
	}
	two := edit.Obj{"label": edit.Str("Two")}
	idThree := edit.Obj{"label": edit.Str("Three"), "code": edit.Int(3)}
	cases := []struct {
		op   edit.Operation
		want []edit.Locked
	}{
		{edit.Operation{Kind: edit.OpAddEntry, Path: "ids", Key: edit.Key("three"), Value: idThree}, []edit.Locked{{Name: "p.ids", Key: "three"}}},
		{edit.Operation{Kind: edit.OpSet, Path: "ids.two.code", Value: edit.Int(5)}, []edit.Locked{{Name: "p.ids", Key: "two"}}},
		{edit.Operation{Kind: edit.OpSet, Path: "ids.one.code", Value: edit.Int(5)}, nil},
		{edit.Operation{Kind: edit.OpRetire, Path: "ids.one"}, []edit.Locked{{Name: "p.ids", Key: "one"}}},
		{edit.Operation{Kind: edit.OpRetire, Path: "Code.b"}, []edit.Locked{{Name: "p.Code", Key: "b"}}},
		{edit.Operation{Kind: edit.OpAddEntry, Path: "rows", Key: edit.Key("y"), Value: two}, nil},
		{edit.Operation{Kind: edit.OpSet, Path: "ids.one.label", Value: edit.Str("Uno")}, nil},
	}
	env := edit.Env{Project: p, Host: hostOf}
	for _, c := range cases {
		plan, err := edit.Apply(context.Background(), env, edit.NewSnapshot(a), edit.Request{Ops: []edit.Operation{c.op}})
		if err != nil || !slices.Equal(plan.Locked, c.want) {
			t.Errorf("%d %s: %v, %v, want %v", c.op.Kind, c.op.Path, plan, err, c.want)
		}
	}
}
