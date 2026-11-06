package canon_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// dirValid is a d/ts/b.json that decodes: with it the table has a value.
const dirValid = "{\n  \"v\": 2\n}\n"

// undoOf applies e to a project opened with law, and returns the result's Undo as JSON, or the error.
func undoOf(t *testing.T, law map[string]string, e canon.Edit) (string, error) {
	t.Helper()
	p, _ := openEdit(t, law)
	res, err := p.Edit(context.Background(), e)
	if err != nil {
		return "", err
	}
	out, err := json.Marshal(res.Undo)
	return string(out), err
}

// API.md E23: an old value comes back in one form whether its table has a value or not: a Source
// literal printed from the decoded value.
func TestEditPoisonedDirUndoForm(t *testing.T) {
	for _, op := range []canon.Op{
		canon.Remove("d:ts.a"), canon.Set("d:ts.a.v", canon.Int(5)), canon.Set("d:ts.a", canon.Obj{"v": canon.Int(4)}), canon.Reset("d:ts.a.w"),
	} {
		e := canon.Edit{AllowErrors: true, Ops: []canon.Op{op}}
		valued := dirLaw(dirValid)
		valued["d/ts/a.json"] = dirWide
		poisoned := dirLaw(dirBroken[0].content)
		poisoned["d/ts/a.json"] = dirWide
		want, werr := undoOf(t, valued, e)
		got, gerr := undoOf(t, poisoned, e)
		if werr != nil || gerr != nil || got != want {
			t.Errorf("API.md E23, %s %s: poisoned Undo %s (%v), valued %s (%v)", op.Kind, op.Path, got, gerr, want, werr)
		}
	}
}

// dirRetired is a retired entry file the lock does not hold (WIRE.md 5.7, 6.5).
const dirRetired = "{\n  \"$retired\": true,\n  \"v\": 1\n}\n"

// API.md E23, E22, N6: removing a retired entry of a stable load.dir table, with a value or
// without one, gives an Undo that adds it back and retires it again, so its file comes back.
func TestEditPoisonedDirRetired(t *testing.T) {
	for _, b := range []string{dirValid, "{}\n"} {
		law := dirLaw(b)
		law["d/d.canon"] = dirRecord + "let ts: stable table T = " + dirCall
		law["d/canon.lock"] = "# canon.lock v1\ntable  d.ts  a\n"
		law["d/ts/c.json"] = dirRetired
		p, m := openEdit(t, law)
		res, err := p.Edit(context.Background(), canon.Edit{AllowErrors: true, Ops: []canon.Op{canon.Remove("d:ts.c")}})
		if err != nil || !res.Applied {
			t.Errorf("API.md E19, b %q: Remove: %+v, %v", b, res, err)
			continue
		}
		sameUndo(t, "API.md E23, b "+b, res.Undo, []canon.Op{canon.AddEntry("d:ts", canon.Key("c"), canon.Source("{ v: 1 }")), canon.Retire("d:ts.c")})
		undone(t, "API.md E22, b "+b, p, res)
		if got := lockOf(t, m, "d/ts/c.json"); got != dirRetired {
			t.Errorf("API.md E22, b %q: c.json %q, want %q", b, got, dirRetired)
		}
	}
}

// API.md M9; FORMATTER.md 14.1: a file whose numbers are not in their typed-canonical text is
// judged so by its fields' declared types whether its table has a value or not.
func TestEditPoisonedDirNumbers(t *testing.T) {
	head := "/// D.\npackage d\n\n/// T.\nrecord T {\n  /// V.\n  v: Int\n  /// F.\n  f: Float = 0.5\n}\n\n/// Ts.\nlet ts: table T = " + dirCall
	wide := "{\n  \"v\": 1,\n  \"f\": 1.0\n}\n"
	e := canon.Edit{AllowErrors: true, Ops: []canon.Op{canon.Set("d:ts.a.v", canon.Int(3))}}
	var errs [2]error
	for i, b := range []string{dirValid, dirBroken[1].content} {
		law := map[string]string{"d/d.canon": head, "d/ts/a.json": wide, "d/ts/b.json": b}
		p, m := openEdit(t, law)
		_, errs[i] = p.Edit(context.Background(), e)
		sameFiles(t, "API.md M9", m, law)
	}
	if _, ok := isErr[*canon.NotCanonicalError](errs[1], canon.ErrNotCanonical); !ok || !errors.Is(errs[0], canon.ErrNotCanonical) {
		t.Errorf("API.md M9: poisoned %v, valued %v, want ErrNotCanonical for both", errs[1], errs[0])
	}
}

// API.md E9, N1: file paths order a load.dir's entries, so a Move is NotEditable `order`, with a
// value or without one.
func TestEditPoisonedDirMove(t *testing.T) {
	for _, b := range []string{dirValid, dirBroken[0].content} {
		p, _ := openEdit(t, dirLaw(b))
		_, err := p.Edit(context.Background(), canon.Edit{AllowErrors: true, Ops: []canon.Op{canon.Move("d:ts.a", 1)}})
		ne, ok := isErr[*canon.NotEditableError](err, canon.ErrNotEditable)
		if !ok || ne.Reason != canon.ReasonOrder {
			t.Errorf("API.md E9, b %q: %v, want NotEditable order", b, err)
		}
	}
}
