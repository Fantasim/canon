package canon_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// API.md E23, E20, LOCK.md 6.2 (COMPILER-ISSUES #4): an AddEntry missing a required field leaves its
// table valueless, its Undo a Remove of the new key, never locked; a complete one takes a Retire (a
// Set swapping it for an incomplete entry, whose Undo is dead: TestEditUndoIncompleteDeadRefused).
func TestEditUndoIncompleteEntry(t *testing.T) {
	add := func(table, key string, v canon.Lit) canon.Op { return canon.AddEntry(table, canon.Key(key), v) }
	label := canon.Obj{"label": canon.Str("S")}
	retired := lockFirst + "table  a.codes  second\n"
	cases := []struct {
		name string
		ops  []canon.Op
		want []canon.Op
		lock string
	}{
		{"stable, incomplete", []canon.Op{add("a:codes", "second", canon.Obj{})},
			[]canon.Op{canon.Remove("a:codes.second")}, lockFirst},
		{"stable, complete", []canon.Op{add("a:codes", "second", label)},
			[]canon.Op{{Kind: canon.OpRetire, Path: "a:codes.second"}}, retired},
		{"plain, incomplete", []canon.Op{add("a:statuses", "second", canon.Obj{})},
			[]canon.Op{canon.Remove("a:statuses.second")}, lockFirst},
		{"stable, complete beside incomplete", []canon.Op{add("a:codes", "second", label), add("a:codes", "third", canon.Obj{})},
			[]canon.Op{canon.Remove("a:codes.third"), canon.Remove("a:codes.second")}, lockFirst},
		{"stable, incomplete, then another root's Set", []canon.Op{add("a:codes", "second", canon.Obj{}),
			canon.Set("a:config.port", canon.Int(1))}, []canon.Op{{Kind: canon.OpReset, Path: "a:config.port"},
			canon.Remove("a:codes.second")}, lockFirst},
		{"stable, complete, then a Set dropping it", []canon.Op{add("a:codes", "second", label),
			canon.Set("a:codes", canon.Source(`{ first { label: "One" } }`))},
			[]canon.Op{canon.Set("a:codes", canon.Source(""))}, lockFirst},
	}
	for _, c := range cases {
		p, m := openEdit(t)
		res, err := p.Edit(context.Background(), canon.Edit{AllowErrors: true, Ops: c.ops})
		if err != nil || !res.Applied {
			t.Errorf("%s: Edit: %+v, %v", c.name, res, err)
			continue
		}
		if !slices.EqualFunc(res.Undo, c.want, sameOp) {
			t.Errorf("API.md E23, %s: Undo %+v, want %+v", c.name, res.Undo, c.want)
		}
		if got := lockOf(t, m, "a/canon.lock"); got != c.lock {
			t.Errorf("API.md E20, %s: lock %q, want %q", c.name, got, c.lock)
		}
	}
}

// API.md E22, E23 (DECISIONS 273, 466): a Set dropping the new entry for an incomplete one leaves
// the stable table valueless, and its Undo, a Set of the whole table, no source repair applies
// (ErrNoValue): with AllowErrors the edit is refused so, and neither file is written.
func TestEditUndoIncompleteDeadRefused(t *testing.T) {
	p, m := openEdit(t)
	src := read(t, m, "a/a.canon")
	_, err := p.Edit(context.Background(), canon.Edit{AllowErrors: true, Ops: []canon.Op{
		canon.AddEntry("a:codes", canon.Key("second"), canon.Obj{"label": canon.Str("S")}),
		canon.Set("a:codes", canon.Source(`{ first { label: "First" }, third {} }`)),
	}})
	if !errors.Is(err, canon.ErrNoValue) || read(t, m, "a/a.canon") != src || lockOf(t, m, "a/canon.lock") != lockFirst {
		t.Errorf("AddEntry codes.second, then a Set to an incomplete third, with AllowErrors: %v, want ErrNoValue, nothing written", err)
	}
}
