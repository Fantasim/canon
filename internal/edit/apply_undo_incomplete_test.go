package edit_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// incompleteSrc is COMPILER-ISSUES #4's law: a stable table whose entries need hp.
const incompleteSrc = "package d\n\nrecord M {\n  hp: Int\n}\n\nlet ms: stable table M = {\n  wolf { hp: 1 }\n}\n"

// API.md E23, E20 (COMPILER-ISSUES #4): with AllowErrors, an AddEntry of an entry missing hp leaves
// the table without a value; the Undo still names the new key, read from what the operation added:
// a Remove (its id was never locked), in a stable table as in a plain one; a complete one a Retire.
func TestUndoIncompleteEntry(t *testing.T) {
	for _, c := range []struct {
		name, src, value string
		want             edit.Op
	}{
		{"stable, incomplete", incompleteSrc, "{}", edit.OpRemove},
		{"plain, incomplete", strings.Replace(incompleteSrc, "stable ", "", 1), "{}", edit.OpRemove},
		{"stable, complete", incompleteSrc, "{ hp: 2 }", edit.OpRetire},
	} {
		fs := srcFS(c.src)()
		fs["law/d/canon.lock"] = file("# canon.lock v1\ntable  d.ms  wolf\n")
		s := open(t, fs, nil, "", "d")
		add := edit.Operation{Kind: edit.OpAddEntry, Path: "d:ms", Key: edit.Key("bat"), Value: edit.Source(c.value)}
		plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{add}, AllowErrors: true})
		if err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if want := []edit.Operation{{Kind: c.want, Path: "d:ms.bat"}}; !slices.EqualFunc(plan.Undo, want, func(x, y edit.Operation) bool {
			return x.Kind == y.Kind && x.Path == y.Path
		}) {
			t.Errorf("API.md E23, %s: Undo %+v, want %+v", c.name, plan.Undo, want)
		}
	}
}
