package canon_test

import (
	"context"
	"errors"
	"testing"
	"time"

	canon "github.com/fantasim/canonlang/api"
)

// quiet is how long a watch stays silent before a test takes its events as complete: past
// W14's one-second cap, so an external event for the edit's own writes would have come.
const quiet = 1500 * time.Millisecond

// API.md E17, API.md E18, API.md W15, API.md S10, API.md W7, API.md V14: an edit writes its file,
// re-checks its package and importers (a, b; not c), evaluates after it, publishes one event with
// cause edit and the result's revision, which the project reads next.
func TestEditApplies(t *testing.T) {
	p, m := openEdit(t, nil)
	events, _ := watch(t, p, nil)
	e := editPort(p.Revision())
	e.Evaluate = []string{"a:config"}
	res, err := p.Edit(context.Background(), e)
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if ev := res.Eval["a:config"]; ev == nil || ev.Revision != res.Revision || ev.Path != "a:config" {
		t.Errorf("API.md V14: %+v", ev)
	}
	if !res.Applied || len(res.Changes) != 1 || res.Changes[0].Path != "a/a.canon" || res.Changes[0].Kind != canon.Modified {
		t.Fatalf("result: %+v", res)
	}
	if c := res.Changes[0]; c.Before != nil || c.After != nil {
		t.Error("API.md §8.1: Before and After are for DryRun only")
	}
	if got := lines(read(t, m, "a/a.canon"), "let config"); len(got) != 1 || got[0] != "let config: Config = { port: 9000 }" {
		t.Errorf("API.md W7: %q", got)
	}
	if res.Summary.Packages != 2 || res.Summary.Errors != 0 || len(res.Findings) != 0 {
		t.Errorf("API.md E17: summary %+v, findings %v", res.Summary, res.Findings)
	}
	if rev := p.Revision(); rev != res.Revision {
		t.Errorf("API.md S10: revision %s after the edit, result %s", rev, res.Revision)
	}
	if ev := nextEvent(t, events); ev.Cause != canon.CauseEdit || ev.Revision != res.Revision || ev.Err != nil {
		t.Errorf("API.md W15: event %s %s %v, result %s", ev.Cause, ev.Revision, ev.Err, res.Revision)
	}
	silent(t, events, quiet, "API.md W15: after the edit's one event")
}

// API.md S12: an edit refuses to write a file that has an overlay, naming it; nothing is written.
func TestEditOverlay(t *testing.T) {
	p, m := openEdit(t, nil)
	before := read(t, m, "a/a.canon")
	if err := p.SetOverlay("a/a.canon", []byte(before)); err != nil {
		t.Fatal(err)
	}
	_, err := p.Edit(context.Background(), editPort(""))
	pe, ok := isErr[*canon.PathError](err, canon.ErrOverlay)
	if !ok || pe.Path != "a/a.canon" || pe.Op != -1 {
		t.Fatalf("Edit: %v", err)
	}
	if read(t, m, "a/a.canon") != before {
		t.Error("API.md S12: the file was written")
	}
	if err := p.ClearOverlay("a/a.canon"); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Edit(context.Background(), editPort("")); err != nil {
		t.Errorf("API.md S12: the overlay cleared, the edit is refused: %v", err)
	}
}

// API.md W15, API.md E21: DryRun computes the edit, Before and After included, and writes
// nothing: not Applied, the base revision, no event.
func TestEditDryRun(t *testing.T) {
	p, m := openEdit(t, nil)
	events, _ := watch(t, p, nil)
	before, base := read(t, m, "a/a.canon"), p.Revision()
	e := editPort(base)
	e.DryRun = true
	res, err := p.Edit(context.Background(), e)
	if err != nil || res.Applied || res.Revision != base || len(res.Changes) != 1 {
		t.Fatalf("Edit: %+v, %v", res, err)
	}
	c := res.Changes[0]
	if string(c.Before) != before || len(lines(string(c.After), "{ port: 9000 }")) != 1 {
		t.Errorf("DryRun: Before %q After %q", c.Before, c.After)
	}
	if read(t, m, "a/a.canon") != before || p.Revision() != base {
		t.Error("DryRun wrote")
	}
	silent(t, events, eventNone, "DryRun")
}

// API.md E19: an edit whose result has an error writes nothing and returns its result with a
// *RejectedError holding the errors; with AllowErrors it is written, errors and all.
func TestEditRejected(t *testing.T) {
	p, m := openEdit(t, nil)
	before := read(t, m, "a/a.canon")
	e := canon.Edit{Ops: []canon.Op{canon.Set("a:statuses.open.weight", canon.Int(-1))}}
	res, err := p.Edit(context.Background(), e)
	re, ok := isErr[*canon.RejectedError](err, canon.ErrRejected)
	if !ok || res == nil || res.Applied || len(re.Findings) != 1 || res.Summary.Errors != 1 {
		t.Fatalf("Edit: %+v, %v", res, err)
	}
	if f := re.Findings[0]; f.Path != "statuses.open" || f.Package != "a" || f.Severity != canon.SeverityError {
		t.Errorf("API.md E19: finding %+v", f)
	}
	if read(t, m, "a/a.canon") != before {
		t.Error("API.md E19: a rejected edit wrote")
	}
	e.AllowErrors = true
	res, err = p.Edit(context.Background(), e)
	if err != nil || !res.Applied || res.Summary.Errors != 1 {
		t.Fatalf("AllowErrors: %+v, %v", res, err)
	}
	if len(lines(read(t, m, "a/a.canon"), "weight: -1")) != 1 {
		t.Error("API.md E19: AllowErrors did not write")
	}
}

// API.md M9, API.md E21: a file not in canonical layout is refused, named, unless Normalize,
// which formats it whole as part of the edit.
func TestEditNotCanonical(t *testing.T) {
	loose := "/// C.\npackage c\n\n/// N.\nlet n: Int =  1\n"
	p, m := openEdit(t, map[string]string{"c/c.canon": loose})
	e := canon.Edit{Ops: []canon.Op{canon.Set("c:n", canon.Int(2))}}
	_, err := p.Edit(context.Background(), e)
	if nc, ok := isErr[*canon.NotCanonicalError](err, canon.ErrNotCanonical); !ok || len(nc.Files) != 1 || nc.Files[0] != "c/c.canon" {
		t.Fatalf("Edit: %v", err)
	}
	if read(t, m, "c/c.canon") != loose {
		t.Error("API.md M9: a refused edit wrote")
	}
	e.Normalize = true
	if _, err := p.Edit(context.Background(), e); err != nil {
		t.Fatalf("Normalize: %v", err)
	}
	if got := read(t, m, "c/c.canon"); got != "/// C.\npackage c\n\n/// N.\nlet n: Int = 2\n" {
		t.Errorf("API.md M9: %q", got)
	}
}

// API.md E22, API.md E23: Undo, applied at the result's revision, restores the value; a field
// set where it was absent is reset, so the file is as before.
func TestEditUndo(t *testing.T) {
	p, m := openEdit(t, nil)
	before := read(t, m, "a/a.canon")
	res, err := p.Edit(context.Background(), editPort(p.Revision()))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Undo) != 1 || res.Undo[0].Kind != canon.OpReset || res.Undo[0].Path != "a:config.port" {
		t.Fatalf("API.md E23: Undo %+v", res.Undo)
	}
	if _, err := p.Edit(context.Background(), canon.Edit{Base: res.Revision, Ops: res.Undo}); err != nil {
		t.Fatalf("Undo: %v", err)
	}
	if read(t, m, "a/a.canon") != before {
		t.Error("API.md E22: Undo did not restore the file")
	}
}

// API.md V14, API.md E19: Edit.Evaluate evaluates each path on the sources after the edit, with
// the result's revision; a DryRun with AllowErrors shows the error it would write.
func TestEditEvaluate(t *testing.T) {
	p, _ := openEdit(t, nil)
	e := canon.Edit{
		Ops:         []canon.Op{canon.Set("a:statuses.open.weight", canon.Int(-1))},
		AllowErrors: true, DryRun: true, Evaluate: []string{"a:statuses.open", "a:config"},
	}
	res, err := p.Edit(context.Background(), e)
	if err != nil || len(res.Eval) != len(e.Evaluate) {
		t.Fatalf("Edit: %+v, %v", res, err)
	}
	open := res.Eval["a:statuses.open"]
	if open == nil || open.Path != "a:statuses.open" || open.Revision != res.Revision || len(open.Findings) != 1 {
		t.Fatalf("API.md V14: %+v", open)
	}
	if open.Findings[0].Path != "statuses.open" || res.Eval["a:config"].Title.Value != "config" {
		t.Errorf("API.md V14: %+v %+v", open.Findings[0], res.Eval["a:config"].Title)
	}
	e.Evaluate = []string{"a:config..x"}
	if _, err := p.Edit(context.Background(), e); !errors.Is(err, canon.ErrBadPath) {
		t.Errorf("API.md E21: a bad Evaluate path: %v", err)
	}
}
