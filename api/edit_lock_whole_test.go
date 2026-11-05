package canon_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/internal/diag"
)

// wholeCodes is a whole-table Set of a:codes that keeps first and adds a complete entry, third.
const wholeCodes = `{ first { label: "One" }, third { label: "T" } }`

// codesBlock is the text of a:codes in the source after an edit.
func codesBlock(t *testing.T, m *memFS) string {
	t.Helper()
	src := read(t, m, "a/a.canon")
	from := strings.Index(src, "let codes")
	to := strings.Index(src, "/// The settings.")
	if from < 0 || to < from {
		t.Fatalf("no codes in %q", src)
	}
	return src[from:to]
}

// API.md E20, DECISIONS 232: a whole-table Set that adds a complete entry to a stable table locks
// its id in the same edit, as an AddEntry does (with or without AllowErrors); a later Remove of
// that id is refused (E4).
func TestEditLockWholeSetAdds(t *testing.T) {
	want := lockFirst + "table  a.codes  third\n"
	for _, allow := range []bool{false, true} {
		p, m := openEdit(t)
		res, err := p.Edit(context.Background(), canon.Edit{AllowErrors: allow, Ops: []canon.Op{canon.Set("a:codes", canon.Source(wholeCodes))}})
		if err != nil || !res.Applied {
			t.Fatalf("allow %v: %+v, %v", allow, res, err)
		}
		if got := lockOf(t, m, "a/canon.lock"); got != want {
			t.Errorf("DECISIONS 232, allow %v: lock %q, want %q", allow, got, want)
		}
		_, err = p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{canon.Remove("a:codes.third")}})
		if !errors.Is(err, canon.ErrStableKey) {
			t.Errorf("API.md E4, allow %v: Remove of a:codes.third: %v", allow, err)
		}
	}
}

// API.md E20, E22, DECISIONS 474, 476: an entry the whole Set adds that does not evaluate leaves the
// table without a value, so its whole-table Undo is dead and the edit is refused under AllowErrors.
func TestEditLockWholeSetIncomplete(t *testing.T) {
	p, m := openEdit(t)
	src := read(t, m, "a/a.canon")
	_, err := p.Edit(context.Background(), canon.Edit{AllowErrors: true, Ops: []canon.Op{
		canon.Set("a:codes", canon.Source(`{ first { label: "First" }, third {} }`)),
	}})
	if !errors.Is(err, canon.ErrNoValue) {
		t.Fatalf("DECISIONS 474: Edit: %v, want ErrNoValue", err)
	}
	if got := lockOf(t, m, "a/canon.lock"); got != lockFirst || read(t, m, "a/a.canon") != src {
		t.Errorf("DECISIONS 474: a refused edit wrote (lock %q)", got)
	}
}

// API.md E20, DECISIONS 232: an id the whole Set adds whose @stable value the lock holds for
// another id is skipped; the other id it adds is locked, an entry the table held (two) is not an id
// the edit adds, and the finding stays.
func TestEditLockWholeSetConflict(t *testing.T) {
	p, m := openEdit(t, stableLaw)
	res, err := p.Edit(context.Background(), canon.Edit{AllowErrors: true, Ops: []canon.Op{
		canon.Set("f:ids", canon.Source(`{ one { label: "One", code: 1 }, two { label: "Two", code: 2 }, three { label: "T", code: 1 }, four { label: "F", code: 4 } }`)),
	}})
	if err != nil || !res.Applied || !slices.ContainsFunc(res.Findings, isCode(diag.E6002.Def().Code)) {
		t.Fatalf("Edit: %+v, %v", res, err)
	}
	want := lockHeader + "field  f.ids.code  1  one\nfield  f.ids.code  4  four\ntable  f.ids  four\ntable  f.ids  one\n"
	if got := lockOf(t, m, "f/canon.lock"); got != want {
		t.Errorf("DECISIONS 232: lock %q, want %q", got, want)
	}
}

// API.md E20, E22, E23, LOCK.md §4.1: the Undo of a whole-table Set restores the source, never the lock.
func TestEditLockWholeSetUndo(t *testing.T) {
	p, m := openEdit(t)
	res, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{canon.Set("a:codes", canon.Source(wholeCodes))}})
	if err != nil || !res.Applied {
		t.Fatalf("Edit: %+v, %v", res, err)
	}
	if _, err := p.Edit(context.Background(), canon.Edit{Base: res.Revision, Ops: res.Undo}); err != nil {
		t.Fatalf("Undo %+v: %v", res.Undo, err)
	}
	if got, want := lockOf(t, m, "a/canon.lock"), lockFirst+"table  a.codes  third  retired\n"; got != want {
		t.Errorf("LOCK.md §4.1: lock %q after the Undo, want %q", got, want)
	}
	block := codesBlock(t, m)
	if !strings.Contains(block, `first { label: "First" }`) || !strings.Contains(block, "retired third") {
		t.Errorf("API.md E22, E23: source after the Undo: %q", block)
	}
	if _, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{canon.Remove("a:codes.third")}}); !errors.Is(err, canon.ErrStableKey) {
		t.Errorf("API.md E4: Remove after the Undo: %v", err)
	}
}

// loadedCodes is editLaw with a:codes read by load from a canonical JSON file, first locked.
var loadedCodes = map[string]string{
	"a/a.canon": strings.Replace(editLaw["a/a.canon"], "let codes: stable table Code = {\n  first { label: \"First\" }\n}",
		"let codes: stable table Code = load(\"c.json\")", 1),
	"a/c.json": "{\n  \"first\": {\n    \"label\": \"First\"\n  }\n}\n",
}

// API.md E4, E20, DECISIONS 232: a whole-table Set that drops an id the lock holds is refused with
// ErrStableKey, at its own operation, with or without AllowErrors; one that drops a pending id is allowed.
func TestEditLockWholeSetDrops(t *testing.T) {
	port := canon.Set("a:config.port", canon.Int(9000))
	cases := []struct {
		name, dir string
		law       map[string]string
		before    []canon.Op // operations ahead of the Set
		path, src string
		refused   bool
	}{
		{"a held id", "a", nil, nil, "a:codes", `{ third { label: "T" } }`, true},
		{"a held id, after another operation", "a", nil, []canon.Op{port}, "a:codes", `{ third { label: "T" } }`, true},
		{"a held id in a table read by load", "a", loadedCodes, nil, "a:codes", `{ third { label: "T" } }`, true},
		{"a held id kept beside a new one", "a", nil, nil, "a:codes", wholeCodes, false},
		{"a pending id", "f", stableLaw, nil, "f:ids", `{ one { label: "One", code: 1 } }`, false},
	}
	for _, c := range cases {
		for _, allow := range []bool{false, true} {
			p, m := openEdit(t, c.law)
			lock, src := lockOf(t, m, c.dir+"/canon.lock"), read(t, m, c.dir+"/"+c.dir+".canon")
			ops := append(slices.Clone(c.before), canon.Set(c.path, canon.Source(c.src)))
			_, err := p.Edit(context.Background(), canon.Edit{AllowErrors: allow, Ops: ops})
			var pe *canon.PathError
			if c.refused != errors.Is(err, canon.ErrStableKey) || !c.refused && err != nil {
				t.Errorf("API.md E4, %s, allow %v: %v", c.name, allow, err)
			}
			if c.refused && (!errors.As(err, &pe) || pe.Op != len(c.before) || pe.Path != c.path) {
				t.Errorf("API.md E4, %s, allow %v: error %#v, want op %d at %s", c.name, allow, err, len(c.before), c.path)
			}
			if c.refused && (lockOf(t, m, c.dir+"/canon.lock") != lock || read(t, m, c.dir+"/"+c.dir+".canon") != src) {
				t.Errorf("API.md E4, %s, allow %v: a refused edit wrote", c.name, allow)
			}
		}
	}
}
