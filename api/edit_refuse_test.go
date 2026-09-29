package canon_test

import (
	"context"
	"errors"
	"io/fs"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// API.md E21, API.md E1, API.md E2, API.md V1, API.md W5, API.md E3, API.md E4, API.md X1: each
// operation's refusal is its error type, with the op's index and its path as given; the first
// failing op decides.
func TestEditRefusals(t *testing.T) {
	p, m := openEdit(t, nil)
	before := read(t, m, "a/a.canon")
	port := canon.Set("a:config.port", canon.Int(1))
	cases := []struct {
		name string
		ops  []canon.Op
		want error
		op   int
	}{
		{"E21 syntax before a later bad kind", []canon.Op{canon.Set("a:config..x", canon.Int(1)), {Kind: "bogus"}}, canon.ErrBadPath, 0},
		{"E2 unknown kind", []canon.Op{port, {Kind: "bogus", Path: "a:config"}}, canon.ErrBadOp, 1},
		{"E2 Add on a table", []canon.Op{canon.Add("a:statuses", canon.Obj{})}, canon.ErrBadOp, 0},
		{"P6 no root", []canon.Op{port, canon.Set("a:nowhere", canon.Int(1))}, canon.ErrNoPath, 1},
		{"V1 a string for an Int", []canon.Op{canon.Set("a:config.port", canon.Str("x"))}, canon.ErrBadValue, 0},
		{"W5 a computed value", []canon.Op{canon.Set("a:twice", canon.Int(1))}, canon.ErrNotEditable, 0},
		{"E3 a key taken", []canon.Op{canon.AddEntry("a:statuses", canon.Key("open"), canon.Obj{"label": canon.Str("O")})}, canon.ErrKeyExists, 0},
		{"E4 Remove of a stable id", []canon.Op{canon.Remove("a:codes.first")}, canon.ErrStableKey, 0},
		{"E4 Unretire", []canon.Op{canon.Unretire("a:codes.first")}, canon.ErrStableKey, 0},
	}
	for _, c := range cases {
		_, err := p.Edit(context.Background(), canon.Edit{Ops: c.ops})
		if !errors.Is(err, c.want) || opOf(err) != c.op || !strings.HasPrefix(err.Error(), "op ") {
			t.Errorf("%s: %v, want %v at op %d", c.name, err, c.want, c.op)
		}
	}
	var ve *canon.ValueError
	_, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{canon.Set("a:config.port", canon.Str("x"))}})
	if !errors.As(err, &ve) || ve.Path != "a:config.port" || ve.Expected != "Int" {
		t.Errorf("API.md V1: %+v", ve)
	}
	var ne *canon.NotEditableError
	_, err = p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{canon.Set("a:twice", canon.Int(1))}})
	if !errors.As(err, &ne) || ne.Reason != canon.ReasonComputed || ne.Path != "a:twice" {
		t.Errorf("API.md W5: %+v", ne)
	}
	if read(t, m, "a/a.canon") != before {
		t.Error("API.md E21: a refused edit wrote")
	}
}

// opOf is the Op of an error of API.md §15, -2 for another error.
func opOf(err error) int {
	var (
		pe *canon.PathError
		ve *canon.ValueError
		ne *canon.NotEditableError
	)
	switch {
	case errors.As(err, &pe):
		return pe.Op
	case errors.As(err, &ve):
		return ve.Op
	case errors.As(err, &ne):
		return ne.Op
	}
	return -2
}

// API.md E12: a rename that would change a reference that is not editable fails whole with a
// *NotEditableError whose Detail names it.
func TestEditRenameComputedRef(t *testing.T) {
	src := editLaw["a/a.canon"] + "\n/// Done, computed.\nlet pd: ref statuses = statuses.done\n"
	p, m := openEdit(t, map[string]string{"a/a.canon": src})
	_, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{canon.Rename("a:statuses.done", canon.Key("closed"))}})
	ne, ok := isErr[*canon.NotEditableError](err, canon.ErrNotEditable)
	if !ok || ne.Op != 0 || !strings.Contains(ne.Detail, "pd") {
		t.Fatalf("Edit: %v", err)
	}
	if read(t, m, "a/a.canon") != src {
		t.Error("API.md E12: a refused rename wrote")
	}
}

// API.md S5, API.md S6, API.md S4: an edit is stale when a source changed since its base (any
// source can reshape the import graph), never for a file no package reads; Base "" skips the
// check; a revision the project never produced is stale.
func TestEditStale(t *testing.T) {
	p, m := openEdit(t, nil)
	base := p.Revision()
	writeLaw(t, m, "a/notes.txt", "not read\n")
	res, err := p.Edit(context.Background(), canon.Edit{Base: base, Ops: []canon.Op{canon.Set("c:n", canon.Int(2))}, DryRun: true})
	if err != nil || len(res.Changes) != 1 {
		t.Fatalf("API.md S5: an unread file made the edit stale: %v", err)
	}
	writeLaw(t, m, "c/c.canon", strings.Replace(editLaw["c/c.canon"], "= 1", "= 3", 1))
	_, err = p.Edit(context.Background(), editPort(base))
	if se, ok := isErr[*canon.StaleError](err, canon.ErrStale); !ok || len(se.Files) != 1 || se.Files[0] != "c/c.canon" {
		t.Errorf("API.md S5: %v", err)
	}
	if _, err := p.Edit(context.Background(), editPort("r1:00")); !errors.Is(err, canon.ErrStale) {
		t.Errorf("API.md S4: an unknown base: %v", err)
	}
	if _, err := p.Edit(context.Background(), editPort("")); err != nil {
		t.Errorf("API.md S6: Base \"\" is never stale: %v", err)
	}
}

// changingFS changes a file on disk the first time the journal's directory is listed: after the
// edit read its files, before it writes any.
type changingFS struct {
	*memFS
	armed  atomic.Bool
	once   sync.Once
	change func()
}

func (c *changingFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if c.armed.Load() && strings.HasSuffix(name, "/.canon/journal") {
		c.once.Do(c.change)
	}
	return c.memFS.ReadDir(name)
}

// API.md N9, API.md S11: a file changed on disk while the edit ran is a *StaleError naming it,
// and the edit writes nothing over it.
func TestEditChangedWhileRunning(t *testing.T) {
	theirs := strings.Replace(editLaw["a/a.canon"], "\"Open\"", "\"Opened\"", 1)
	fsys := &changingFS{memFS: lawFS()}
	fsys.change = func() { _ = fsys.memFS.WriteFile("/law/a/a.canon", []byte(theirs)) }
	p, err := canon.Open("/law", canon.Options{FS: fsys})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	fsys.armed.Store(true) // Open lists the journal's directory too (O5)
	_, err = p.Edit(context.Background(), editPort(""))
	if se, ok := isErr[*canon.StaleError](err, canon.ErrStale); !ok || len(se.Files) != 1 || se.Files[0] != "a/a.canon" {
		t.Fatalf("Edit: %v", err)
	}
	if got, _ := fsys.ReadFile("/law/a/a.canon"); string(got) != theirs {
		t.Error("API.md N9: the edit wrote over a change it did not read")
	}
}

// API.md R6, API.md E21 (log-2026-09-29 M4 U5b-r): an op on a value that could not be computed
// is ErrNoValue at its index, explained by the findings of the analysis the edit read it in.
func TestEditNoValue(t *testing.T) {
	p := openValueLaw(t)
	e := canon.Edit{Ops: []canon.Op{canon.Set("a:config.rate", canon.Float(1)), canon.Set("bad", canon.Int(1))}}
	_, err := p.Edit(context.Background(), e)
	var pe *canon.PathError
	if !errors.As(err, &pe) || !errors.Is(err, canon.ErrNoValue) || pe.Op != 1 || len(pe.Findings) != 1 ||
		pe.Findings[0].Severity != canon.SeverityError {
		t.Fatalf("Edit: %v (%+v)", err, pe)
	}
}
