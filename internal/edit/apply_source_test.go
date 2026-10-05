package edit_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// filesBroken is a table whose entries live in their own files, bat missing hp: d:ms has no value.
func filesBroken() mapFS {
	return mapFS{"law/project.canon": file(projectCanon),
		"law/d/d.canon":      file("package d\n\nrecord M {\n  hp: Int\n}\n\n@files(\"m/{id}.canon\")\nlet ms: table M = {}\n"),
		"law/d/m/wolf.canon": file("package d\n\nentry ms.wolf { hp: 1 }\n"),
		"law/d/m/bat.canon":  file("package d\n\nentry ms.bat {}\n"),
	}
}

// API.md E19, E1, N6, E23 (DECISIONS 309): on a table with no value whose entries live in files,
// a Remove of the broken entry deletes its file and a Set of its field writes it, read from the
// sources alone; each Undo takes no Move, the files fixing the order.
func TestSourceOpsEntryFiles(t *testing.T) {
	cases := []struct {
		name string
		op   edit.Operation
		kind edit.ChangeKind
		undo []edit.Op
	}{
		{"Remove", edit.Operation{Kind: edit.OpRemove, Path: "d:ms.bat"}, edit.ChangeDeleted, []edit.Op{edit.OpAddEntry}},
		{"Set", setAt("d:ms.bat.hp", edit.Int(3)), edit.ChangeModified, []edit.Op{edit.OpSet}},
	}
	for _, c := range cases {
		plan, f1, ok := applyIn(t, c.name, filesBroken(), undoView{}, []edit.Operation{c.op})
		if !ok {
			continue
		}
		if len(plan.Changes) != 1 || plan.Changes[0].Path != "d/m/bat.canon" || plan.Changes[0].Kind != c.kind {
			t.Errorf("API.md E19, %s: changes %+v", c.name, plan.Changes)
		}
		if got := opKinds(plan.Undo); !slices.Equal(got, c.undo) {
			t.Errorf("API.md E23, %s: Undo %+v", c.name, plan.Undo)
		}
		if _, f2, ok := applyIn(t, c.name+", Undo", f1, undoView{}, plan.Undo); ok && string(f2["law/d/m/bat.canon"].Data) != "package d\n\nentry ms.bat {}\n" {
			t.Errorf("API.md E22, %s: bat after the Undo: %q", c.name, f2["law/d/m/bat.canon"].Data)
		}
	}
}

// API.md E1, W11 (DECISIONS 309): under an edit layer, an operation on a root with no value stays
// ErrNoValue: an amendment line cannot be written from the sources alone.
func TestSourceOpsEditLayer(t *testing.T) {
	s := open(t, filesBroken(), nil, "dev", "d")
	_, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{{Kind: edit.OpRemove, Path: "d:ms.bat"}}})
	if !errors.Is(err, edit.ErrNoValue) {
		t.Errorf("API.md E1: Remove under an edit layer: %v, want ErrNoValue", err)
	}
}

// opKinds are the kinds of ops.
func opKinds(ops []edit.Operation) []edit.Op {
	out := make([]edit.Op, len(ops))
	for i, op := range ops {
		out[i] = op.Kind
	}
	return out
}
