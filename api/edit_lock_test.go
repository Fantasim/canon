package canon_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/internal/diag"
)

// A package's lock: in step with its sources, missing the pending id `first`, or none at all.
const (
	lockHeader = "# canon.lock v1\n"
	lockFirst  = lockHeader + "table  a.codes  first\n"
	lockNone   = "\x00" // no canon.lock
)

// addSecond adds an id to the stable table a:codes.
var addSecond = canon.AddEntry("a:codes", canon.Key("second"), canon.Obj{"label": canon.Str("Second")})

// API.md E20 (log-2026-09-29 M4 U5b): an edit writes only the lock lines its own changes require,
// appended to the lock as it is: facts already pending in the sources are not the edit's, so an
// unrelated edit never creates or touches a canon.lock; Retire marks its line retired.
func TestEditLockLines(t *testing.T) {
	cases := []struct {
		name, lock string
		op         canon.Op
		want       string // the lock after the edit; lockNone for none
	}{
		{"an id added", lockFirst, addSecond, lockFirst + "table  a.codes  second\n"},
		{"an id added, another pending", lockHeader, addSecond, lockHeader + "table  a.codes  second\n"},
		{"an id added, no lock yet", lockNone, addSecond, lockHeader + "table  a.codes  second\n"},
		{"an id retired", lockFirst, canon.Retire("a:codes.first"), lockHeader + "table  a.codes  first  retired\n"},
		{"unrelated, in step", lockFirst, canon.Set("a:config.port", canon.Int(9000)), lockFirst},
		{"unrelated, one pending", lockHeader, canon.Set("a:config.port", canon.Int(9000)), lockHeader},
		{"unrelated, no lock", lockNone, canon.Set("a:config.port", canon.Int(9000)), lockNone},
	}
	for _, c := range cases {
		p, m := openEdit(t, map[string]string{"a/canon.lock": c.lock})
		if c.lock == lockNone {
			_ = m.Remove("/law/a/canon.lock")
		}
		if _, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{c.op}}); err != nil {
			t.Errorf("%s: %v", c.name, err)
			continue
		}
		if got := lockOf(t, m, "a/canon.lock"); got != c.want {
			t.Errorf("API.md E20, %s: lock %q, want %q", c.name, got, c.want)
		}
	}
}

// weightless is editLaw's package a with its status open failing its check.
var weightless = strings.Replace(editLaw["a/a.canon"], `label: "Open", next`, `label: "Open", weight: -1, next`, 1)

// API.md E20, API.md B4 (log-2026-09-29 M4 U5b-r): an edit that fixes an error and adds an id
// writes the id's line, and the lock is in step with the sources after it.
func TestEditLockFixAndAdd(t *testing.T) {
	p, m := openEdit(t, map[string]string{"a/a.canon": weightless})
	e := canon.Edit{Ops: []canon.Op{canon.Set("a:statuses.open.weight", canon.Int(0)), addSecond}}
	if _, err := p.Edit(context.Background(), e); err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if got := read(t, m, "a/canon.lock"); got != lockFirst+"table  a.codes  second\n" {
		t.Errorf("API.md E20: lock %q", got)
	}
	res, err := p.LockCheck(context.Background(), "a")
	if err != nil || len(res.Findings) != 0 {
		t.Errorf("API.md B4: after the edit %+v, %v", res, err)
	}
}

// API.md E20, API.md E19 (log-2026-09-29 M4 U5b-r): with AllowErrors, an id whose
// entry evaluated has its line written though another package holds an error.
func TestEditLockAllowErrors(t *testing.T) {
	box := "/// B.\npackage b\n\nimport a\n\n/// A box.\nrecord Box {\n  /// Its size.\n  size: Int\n\n" +
		"  check size > 0 else \"empty box\"\n}\n\n/// The box.\nlet box: Box = { size: a.config.port - 8765 }\n"
	p, m := openEdit(t, map[string]string{"b/b.canon": box})
	res, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{addSecond}, AllowErrors: true})
	if err != nil || !res.Applied || res.Summary.Errors != 1 || res.Findings[0].Package != "b" {
		t.Fatalf("Edit: %+v, %v", res, err)
	}
	if got := read(t, m, "a/canon.lock"); got != lockFirst+"table  a.codes  second\n" {
		t.Errorf("API.md E20: lock %q", got)
	}
}

// API.md E21 (log-2026-09-29 M4 U5b): a journal found at commit time, an edit unfinished or
// still running, refuses the edit with ErrProject naming it; nothing is written.
func TestEditJournalAtCommit(t *testing.T) {
	p, m := openEdit(t, nil)
	before := read(t, m, "a/a.canon")
	writeLaw(t, m, ".canon/journal/"+strings.Repeat("ab", 32)+".json", "{}")
	_, err := p.Edit(context.Background(), editPort(""))
	if !errors.Is(err, canon.ErrProject) || !strings.Contains(err.Error(), "unfinished edit journal") ||
		!strings.Contains(err.Error(), strings.Repeat("ab", 32)) {
		t.Fatalf("Edit: %v", err)
	}
	if read(t, m, "a/a.canon") != before {
		t.Error("an edit refused at commit wrote")
	}
}

// linkFS resolves the directory a to another place once armed, as a symbolic link would.
type linkFS struct {
	*memFS
	armed atomic.Bool
}

func (l *linkFS) EvalSymlinks(name string) (string, error) {
	if rest, ok := strings.CutPrefix(name, "/law/a/"); ok && l.armed.Load() {
		return "/elsewhere/a/" + rest, nil
	}
	return name, nil
}

// API.md E21 (log-2026-09-29 M4 U5b): a file reached through a symbolic link refuses the edit
// at commit with ErrProject and edit's reason; nothing is written.
func TestEditThroughLink(t *testing.T) {
	fsys := &linkFS{memFS: lawFS()}
	p, err := canon.Open("/law", canon.Options{FS: fsys})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	before, _ := fsys.ReadFile("/law/a/a.canon")
	fsys.armed.Store(true)
	_, err = p.Edit(context.Background(), editPort(""))
	if !errors.Is(err, canon.ErrProject) || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("Edit: %v", err)
	}
	if after, _ := fsys.ReadFile("/law/a/a.canon"); !slices.Equal(after, before) || len(journals(fsys.memFS)) != 0 {
		t.Error("an edit refused at commit wrote")
	}
}

// stableLaw is a package whose stable table has a @stable field; its entry two, added by hand
// since the last build, is not in canon.lock yet.
var stableLaw = map[string]string{
	"f/f.canon": "/// F.\npackage f\n\n/// An id.\nrecord Id {\n  /// Its label.\n  label: String\n" +
		"  /// Its code, locked.\n  code: Int @stable\n}\n\n/// Ids.\nlet ids: stable table Id = {\n" +
		"  one { label: \"One\", code: 1 }\n  two { label: \"Two\", code: 2 }\n}\n",
	"f/canon.lock": lockHeader + "field  f.ids.code  1  one\ntable  f.ids  one\n",
}

// API.md E20 (log-2026-09-29 M4 U5b-r2): a Set of a @stable field of an entry the lock does not
// hold yet writes that entry's table and field lines in the same edit.
func TestEditLockStableField(t *testing.T) {
	p, m := openEdit(t, stableLaw)
	if _, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{canon.Set("f:ids.two.code", canon.Int(3))}}); err != nil {
		t.Fatalf("Edit: %v", err)
	}
	want := lockHeader + "field  f.ids.code  1  one\nfield  f.ids.code  3  two\ntable  f.ids  one\ntable  f.ids  two\n"
	if got := read(t, m, "f/canon.lock"); got != want {
		t.Errorf("API.md E20: lock %q, want %q", got, want)
	}
}

// codesLaw is an @codes enum whose lock holds a = 1 and b = 2: member c reuses a's code, which
// a lost when removed (E6001), and member b has another code than its locked one (E6002).
var codesLaw = map[string]string{
	"g/g.canon":    "/// G.\npackage g\n\n/// Codes.\nenum Code @codes(UInt8) { b = 3, c = 1 }\n",
	"g/canon.lock": lockHeader + "enum   g.Code  1  a\nenum   g.Code  2  b\n",
}

// API.md E20, API.md E19 (log-2026-09-29 M4 U5b-r3): under AllowErrors an id whose facts conflict
// with the lock is never written: a locked entry's new value, a locked value reused by a new
// entry, an enum member's reused or changed code; the edit applies and the finding stays.
func TestEditLockConflicts(t *testing.T) {
	cases := []struct {
		law  map[string]string
		lock string
		op   canon.Op
		code diag.Code
	}{
		{stableLaw, "f/canon.lock", canon.Set("f:ids.one.code", canon.Int(5)), diag.E6002.Def().Code},
		{stableLaw, "f/canon.lock", canon.AddEntry("f:ids", canon.Key("three"), canon.Obj{"label": canon.Str("T"), "code": canon.Int(1)}), diag.E6002.Def().Code},
		{codesLaw, "g/canon.lock", canon.Retire("g:Code.c"), diag.E6001.Def().Code},
		{codesLaw, "g/canon.lock", canon.Retire("g:Code.b"), diag.E6002.Def().Code},
	}
	for _, c := range cases {
		p, m := openEdit(t, c.law)
		before := read(t, m, c.lock)
		res, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{c.op}, AllowErrors: true})
		if err != nil || !res.Applied || !slices.ContainsFunc(res.Findings, isCode(c.code)) {
			t.Errorf("%s %s: %+v, %v", c.op.Kind, c.op.Path, res, err)
			continue
		}
		if got := read(t, m, c.lock); got != before {
			t.Errorf("API.md E20, %s %s: lock %q, was %q", c.op.Kind, c.op.Path, got, before)
		}
	}
}

// API.md E20, API.md E19 (log-2026-09-29 M4 U5b-r3 addendum): two ids one edit adds that share a
// @stable value are neither written, no winner chosen; the edit applies, the finding (E3102) stays.
func TestEditLockClash(t *testing.T) {
	p, m := openEdit(t, stableLaw)
	before := read(t, m, "f/canon.lock")
	e := canon.Edit{AllowErrors: true, Ops: []canon.Op{
		canon.AddEntry("f:ids", canon.Key("four"), canon.Obj{"label": canon.Str("Four"), "code": canon.Int(7)}),
		canon.AddEntry("f:ids", canon.Key("five"), canon.Obj{"label": canon.Str("Five"), "code": canon.Int(7)}),
	}}
	res, err := p.Edit(context.Background(), e)
	var codes []string
	if res != nil {
		for _, f := range res.Findings {
			codes = append(codes, f.Code)
		}
	}
	if err != nil || !res.Applied || !slices.Contains(codes, string(diag.E3102.Def().Code)) {
		t.Fatalf("Edit: %v, findings %v", err, codes)
	}
	if got := read(t, m, "f/canon.lock"); got != before {
		t.Errorf("API.md E20: lock %q, was %q (findings %v)", got, before, codes)
	}
}
