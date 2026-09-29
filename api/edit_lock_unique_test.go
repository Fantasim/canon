package canon_test

import (
	"context"
	"errors"
	"io/fs"
	"maps"
	"slices"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/internal/diag"
)

// stableNoLock is stableLaw before its first build: no canon.lock, entries one and two unlocked.
var stableNoLock = map[string]string{"f/f.canon": stableLaw["f/f.canon"]}

// stableTwo is stableLaw with its entry two written as two; its lock holds only one.
func stableTwo(two string) map[string]string {
	law := maps.Clone(stableLaw)
	law["f/f.canon"] = strings.Replace(law["f/f.canon"], `  two { label: "Two", code: 2 }`, two, 1)
	return law
}

// fourCode adds to f:ids an entry four whose @stable code is code.
func fourCode(code int64) canon.Op {
	return canon.AddEntry("f:ids", canon.Key("four"), canon.Obj{"label": canon.Str("Four"), "code": canon.Int(code)})
}

// API.md E20, API.md E19, LOCK.md §1, §2.3, §4.4, §4.6 (log-2026-09-29 M4 U7b, B8-r)
func TestEditLockUniqueSources(t *testing.T) {
	cases := []struct {
		name string
		law  map[string]string
		op   canon.Op
		want string // the lock after the edit; lockNone for none
	}{
		{"no lock, a Set takes an unlocked entry's code", stableNoLock, canon.Set("f:ids.two.code", canon.Int(1)), lockNone},
		{"no lock, an Add takes an unlocked entry's code", stableNoLock, fourCode(2), lockNone},
		{"a lock, an Add takes an unlocked entry's code", stableLaw, fourCode(2), stableLaw["f/canon.lock"]},
		{"a lock, an Add takes an unlocked retired entry's code",
			stableTwo(`  two { label: "Two", code: 2 }` + "\n" + `  retired three { label: "Three", code: 4 }`), fourCode(4), stableLaw["f/canon.lock"]},
	}
	for _, c := range cases {
		p, m := openEdit(t, c.law)
		res, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{c.op}, AllowErrors: true})
		if err != nil || !res.Applied || !slices.ContainsFunc(res.Findings, isCode(diag.E3102.Def().Code)) {
			t.Errorf("%s: %+v, %v", c.name, res, err)
			continue
		}
		if got := lockOf(t, m, "f/canon.lock"); got != c.want {
			t.Errorf("API.md E20, LOCK.md §1, %s: lock %q, want %q", c.name, got, c.want)
		}
	}
}

// API.md E20, API.md E19, LOCK.md §1, §4.4 (log-2026-09-29 M4 U7b)
func TestEditLockUniqueWritten(t *testing.T) {
	cases := []struct {
		name string
		law  map[string]string
		op   canon.Op
		want string
	}{
		{"no lock, a Set", stableNoLock, canon.Set("f:ids.two.code", canon.Int(3)),
			lockHeader + "field  f.ids.code  3  two\ntable  f.ids  two\n"},
		{"a lock, an Add", stableLaw, fourCode(4),
			lockHeader + "field  f.ids.code  1  one\nfield  f.ids.code  4  four\ntable  f.ids  four\ntable  f.ids  one\n"},
	}
	for _, c := range cases {
		p, m := openEdit(t, c.law)
		e := canon.Edit{Ops: []canon.Op{c.op, canon.Set("a:statuses.open.weight", canon.Int(-1))}, AllowErrors: true}
		res, err := p.Edit(context.Background(), e)
		if err != nil || !res.Applied || res.Summary.Errors == 0 || slices.ContainsFunc(res.Findings, isCode(diag.E3102.Def().Code)) {
			t.Errorf("%s: %+v, %v", c.name, res, err)
			continue
		}
		if got := lockOf(t, m, "f/canon.lock"); got != c.want {
			t.Errorf("API.md E20, LOCK.md §4.4, %s: lock %q, want %q", c.name, got, c.want)
		}
	}
}

// API.md E20, LOCK.md §4.3, §5 (log-2026-09-29 M4 B8-r)
func TestEditLockRetireHeld(t *testing.T) {
	p, m := openEdit(t, stableTwo(`  two { label: "Two", code: 1 }`))
	res, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{canon.Retire("f:ids.one")}, AllowErrors: true})
	if err != nil || !res.Applied {
		t.Fatalf("Edit: %+v, %v", res, err)
	}
	for _, code := range []diag.Code{diag.E3102.Def().Code, diag.E6002.Def().Code} {
		if !slices.ContainsFunc(res.Findings, isCode(code)) {
			t.Errorf("API.md E19: no %s in %+v", code, res.Findings)
		}
	}
	want := lockHeader + "field  f.ids.code  1  one\ntable  f.ids  one  retired\n"
	if got := lockOf(t, m, "f/canon.lock"); got != want {
		t.Errorf("API.md E20, LOCK.md §5: lock %q, want %q", got, want)
	}
}

// isCode matches a finding of code.
func isCode(code diag.Code) func(canon.Finding) bool {
	return func(f canon.Finding) bool { return f.Code == string(code) }
}

// lockOf is the project file rel on m, or lockNone when it does not exist.
func lockOf(t *testing.T, m *memFS, rel string) string {
	t.Helper()
	data, err := m.ReadFile("/law/" + rel)
	if errors.Is(err, fs.ErrNotExist) {
		return lockNone
	}
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
