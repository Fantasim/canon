package edit_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

const (
	guardDev  = "package d\nlayer dev\n\namend t {\n  stats[0].val: 50\n  marks[0]: c\n  tags[0]: \"DEV\"\n}\n"
	guardData = "law/d/t.json"
)

// guardFS is pairsFS with a plain list of tags in t, which dev amends by position with the bits
// and pairs lists.
func guardFS() mapFS {
	fs := pairsFS()
	fs["law/d/d.canon"] = file(strings.Replace(pairsSrc, "  /// A note.\n  note: String = \"\"\n}", "  /// A note.\n  note: String = \"\"\n  /// Tags.\n  tags: [String] = []\n}", 1))
	fs[guardData] = file(strings.Replace(pairsT, "  \"note\": \"z\"\n", "  \"note\": \"z\",\n  \"tags\": [\n    \"p\",\n    \"q\"\n  ]\n", 1))
	fs["law/d/dev.layer.canon"] = file(guardDev)
	return fs
}

// opName is op's kind, for messages.
func opName(op edit.Operation) string { return fmt.Sprintf("op %d", op.Kind) }

// heldFS is baseLayeredFS with a written map dev adds a key to.
func heldFS() mapFS {
	fs := baseLayeredFS()
	fs[baseLayeredFile] = file(strings.Replace(baseLayeredSrc, "  /// Servers.\n", "  /// Weights.\n  ws: {String: Int} = {}\n  /// Servers.\n", 1) +
		"\n/// Held.\nlet held: Bag = { ws: { \"a\": 1 } }\n")
	fs[baseLayeredDevF] = file(baseLayeredDev + "\namend held {\n  ws[\"b\"]: 2\n}\n")
	return fs
}

// guardVerdict is op's refusal by Editable and by Apply with dev active (ReasonNone: editable,
// applied), the files the edit leaves, and the plan.
func guardVerdict(t *testing.T, fs mapFS, op edit.Operation) (edit.Reason, edit.Reason, *edit.Plan) {
	t.Helper()
	s := open(t, fs, []string{"dev"}, "", "d")
	e, err := s.snap.Editable(resolve(t, fixture{Snapshot: s.snap, files: s.a.Files()}, op.Path), op.Kind, "")
	if err != nil {
		t.Fatal(err)
	}
	plan, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{op}})
	var ne *edit.NotEditableError
	switch {
	case err == nil:
		return e.Reason, edit.ReasonNone, plan
	case errors.As(err, &ne):
		return e.Reason, ne.Reason, nil
	}
	t.Fatalf("%s %s: %v", opName(op), op.Path, err)
	return 0, 0, nil
}

// API.md E22, W10 (EDIT-MATERIALIZE ruling): with dev active, a base Move in a list dev
// amends by position is `layered` for Editable and Apply alike: the Move back would meet dev.
func TestUndoGuardMoveLayered(t *testing.T) {
	ed, ap, _ := guardVerdict(t, guardFS(), edit.Operation{Kind: edit.OpMove, Path: "d:t.tags[1]", Index: 0})
	if ed != edit.ReasonLayered || ap != edit.ReasonLayered {
		t.Errorf("Move t.tags[1] to 0 under dev: Editable %v, Apply %v, want both layered", ed, ap)
	}
}

// API.md E22, E23 (EDIT-MATERIALIZE ruling): an Insert at a position dev amends would leave dev
// amending the new element, which the Remove undoing it cannot remove: Apply refuses it `layered`,
// judging that Remove; an Insert past dev's position applies, its Undo the file back.
func TestUndoGuardInsertJudged(t *testing.T) {
	_, ap, _ := guardVerdict(t, guardFS(), edit.Operation{Kind: edit.OpInsert, Path: "d:t.tags", Index: 0, Value: edit.Str("z")})
	if ap != edit.ReasonLayered {
		t.Errorf("Insert at 0 under dev: Apply %v, want layered (its Undo's Remove of t.tags[0])", ap)
	}
	op := edit.Operation{Kind: edit.OpInsert, Path: "d:t.tags", Index: 1, Value: edit.Str("z")}
	if _, ap, plan := guardVerdict(t, guardFS(), op); ap != edit.ReasonNone || plan == nil {
		t.Errorf("Insert at 1 under dev: Apply %v, want it applied", ap)
	}
	undoIn(t, undoSpec{name: "Insert at 1", fsys: guardFS(), view: undoView{[]string{"dev"}, ""}, ops: []edit.Operation{op},
		views: [][]string{{"dev"}, nil}, files: []string{guardData}})
}

// API.md E22, E3, N1 (EDIT-MATERIALIZE ruling): with dev adding a key to a written map, a base
// AddEntry and a base Remove of the base's own key apply for Editable and Apply alike, each Undo
// giving every value back: the literal's items, not dev's keys, place the new entry.
func TestUndoGuardMapBesideLayerKey(t *testing.T) {
	for _, op := range []edit.Operation{
		{Kind: edit.OpRemove, Path: "d:held.ws[a]"},
		{Kind: edit.OpAddEntry, Path: "d:held.ws", Key: edit.PathKey("c"), Value: edit.Int(3)},
	} {
		if ed, ap, _ := guardVerdict(t, heldFS(), op); ed != edit.ReasonNone || ap != edit.ReasonNone {
			t.Errorf("%s %s under dev: Editable %v, Apply %v, want both editable", opName(op), op.Path, ed, ap)
			continue
		}
		undoIn(t, undoSpec{name: opName(op) + " " + op.Path, fsys: heldFS(), view: undoView{[]string{"dev"}, ""}, ops: []edit.Operation{op},
			views: [][]string{{"dev"}, nil}, files: []string{baseLayeredFile, baseLayeredDevF}})
	}
}

// API.md V1, WIRE.md 5.3 (EDIT-MATERIALIZE ruling): a base Set in a bits list is judged on the
// base list [a, b], which t.json holds, not on dev's [c, b]: c goes in once, and the Undo restores.
func TestUndoGuardBitsJudgedInBase(t *testing.T) {
	op := edit.Operation{Kind: edit.OpSet, Path: "d:t.marks[1]", Value: edit.Member("c")}
	_, ap, plan := guardVerdict(t, guardFS(), op)
	if ap != edit.ReasonNone || plan == nil {
		t.Fatalf("Set t.marks[1] = c under dev: Apply %v, want it applied", ap)
	}
	if got := fileText(written(guardFS(), plan), guardData); !strings.Contains(got, "\"marks\": 5,") {
		t.Errorf("Set t.marks[1] = c under dev writes\n%s\nwant marks 5 (a, c)", got)
	}
	undoIn(t, undoSpec{name: "Set marks[1]", fsys: guardFS(), view: undoView{[]string{"dev"}, ""}, ops: []edit.Operation{op},
		views: [][]string{{"dev"}, nil}, files: []string{guardData}})
}
