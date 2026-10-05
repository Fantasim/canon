package canon_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// poisonedSrc is COMPILER-ISSUES #4's package p: a table of M, whose entries need hp.
const poisonedSrc = "package p\n\nrecord M {\n  hp: Int\n}\n\nlet ms: stable table M = {\n  wolf { hp: 1 }\n}\n"

// poisonedLock is poisonedSrc's lock: wolf held.
const poisonedLock = "# canon.lock v1\ntable  p.ms  wolf\n"

// poisonedLaw is editLaw with package p, its table stable or plain.
func poisonedLaw(stable bool) map[string]string {
	if stable {
		return map[string]string{"p/p.canon": poisonedSrc, "p/canon.lock": poisonedLock}
	}
	return map[string]string{"p/p.canon": strings.Replace(poisonedSrc, "stable ", "", 1)}
}

// poisoned opens poisonedLaw and adds bat, missing hp, with AllowErrors: p:ms has no value.
func poisoned(t *testing.T, stable bool) (*canon.Project, *memFS, *canon.EditResult) {
	t.Helper()
	p, m := openEdit(t, poisonedLaw(stable))
	res, err := p.Edit(context.Background(), canon.Edit{AllowErrors: true, Ops: []canon.Op{
		canon.AddEntry("p:ms", canon.Key("bat"), canon.Obj{}),
	}})
	if err != nil || !res.Applied {
		t.Fatalf("AddEntry: %+v, %v", res, err)
	}
	if _, err := p.Value(context.Background(), "p:ms"); !errors.Is(err, canon.ErrNoValue) {
		t.Fatalf("EVALUATION.md 7.2: p:ms after the AddEntry: %v, want ErrNoValue", err)
	}
	return p, m, res
}

// readable fails the test unless path has a value with text want ("" for any).
func readable(t *testing.T, p *canon.Project, why, path, want string) {
	t.Helper()
	v, err := p.Value(context.Background(), path)
	if err != nil || want != "" && v.Text != want {
		t.Errorf("%s: %s: %+v, %v, want %s", why, path, v, err, want)
	}
}

// API.md E19, E1, DECISIONS 309: on a table poisoned by an entry missing a required field, a
// Remove of that entry works from its source, without AllowErrors, and the table reads again.
func TestEditPoisonedRemove(t *testing.T) {
	for _, stable := range []bool{true, false} {
		p, m, _ := poisoned(t, stable)
		before := read(t, m, "p/p.canon")
		res, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{canon.Remove("p:ms.bat")}})
		if err != nil || !res.Applied {
			t.Errorf("API.md E19, stable %v: Remove: %+v, %v", stable, res, err)
			continue
		}
		readable(t, p, "API.md E19 after the Remove", "p:ms.wolf.hp", "1")
		if got := read(t, m, "p/p.canon"); got != poisonedLaw(stable)["p/p.canon"] {
			t.Errorf("API.md E19, stable %v: source after the Remove:\n%s\nbefore:\n%s", stable, got, before)
		}
		if stable && lockOf(t, m, "p/canon.lock") != poisonedLock {
			t.Errorf("API.md E20, stable %v: lock %q, want it unchanged", stable, lockOf(t, m, "p/canon.lock"))
		}
	}
}

// API.md E19, E1, E20, DECISIONS 309: a Set of the missing field of the poisoned entry works from
// its source, without AllowErrors since it completes the entry; in a stable table it locks bat.
func TestEditPoisonedSetField(t *testing.T) {
	for _, stable := range []bool{true, false} {
		p, m, _ := poisoned(t, stable)
		res, err := p.Edit(context.Background(), canon.Edit{Ops: []canon.Op{canon.Set("p:ms.bat.hp", canon.Int(3))}})
		if err != nil || !res.Applied {
			t.Errorf("API.md E19, stable %v: Set: %+v, %v", stable, res, err)
			continue
		}
		readable(t, p, "API.md E19 after the Set", "p:ms.bat.hp", "3")
		if want := "# canon.lock v1\ntable  p.ms  bat\ntable  p.ms  wolf\n"; stable && lockOf(t, m, "p/canon.lock") != want {
			t.Errorf("API.md E20: lock %q, want %q", lockOf(t, m, "p/canon.lock"), want)
		}
	}
}

// API.md E19, E22, DECISIONS 309: the Undo of the AddEntry, a Remove of bat, applies on the
// poisoned table and gives the source back.
func TestEditPoisonedUndo(t *testing.T) {
	for _, stable := range []bool{true, false} {
		p, m, res := poisoned(t, stable)
		back, err := p.Edit(context.Background(), canon.Edit{Base: res.Revision, Ops: res.Undo})
		if err != nil || !back.Applied {
			t.Errorf("API.md E22, stable %v: the Undo %+v: %+v, %v", stable, res.Undo, back, err)
			continue
		}
		if got := read(t, m, "p/p.canon"); got != poisonedLaw(stable)["p/p.canon"] {
			t.Errorf("API.md E22, stable %v: source after the Undo:\n%s", stable, got)
		}
		readable(t, p, "API.md E22 after the Undo", "p:ms.wolf.hp", "1")
	}
}

// API.md E23, E22, DECISIONS 309: the Undo of each repair gives the poisoned source back, applied
// with AllowErrors since it holds the error again; an id the Set locked stays locked (E23).
func TestEditPoisonedRepairUndo(t *testing.T) {
	cases := []struct {
		name string
		op   canon.Op
		want []canon.Op
	}{
		{"Remove", canon.Remove("p:ms.bat"), []canon.Op{canon.AddEntry("p:ms", canon.Key("bat"), canon.Source("{}"))}},
		{"Set", canon.Set("p:ms.bat.hp", canon.Int(3)), []canon.Op{canon.Set("p:ms.bat", canon.Source("{}"))}},
	}
	for _, c := range cases {
		for _, stable := range []bool{true, false} {
			for _, allow := range []bool{false, true} {
				repairUndone(t, c.name, canon.Edit{AllowErrors: allow, Ops: []canon.Op{c.op}}, stable, c.want)
			}
		}
	}
	two := []canon.Op{canon.Set("p:ms.bat.hp", canon.Int(3)), canon.Set("p:ms.wolf.hp", canon.Int(2))}
	want := []canon.Op{canon.Set("p:ms.wolf.hp", canon.Int(1)), canon.Set("p:ms.bat", canon.Source("{}"))}
	for _, stable := range []bool{true, false} {
		repairUndone(t, "Set, then a Set of a sibling", canon.Edit{Ops: two}, stable, want)
	}
}

// repairUndone applies e to the poisoned project, then its Undo, as TestEditPoisonedRepairUndo says.
func repairUndone(t *testing.T, name string, e canon.Edit, stable bool, want []canon.Op) {
	t.Helper()
	p, m, _ := poisoned(t, stable)
	broken := read(t, m, "p/p.canon")
	name += fmt.Sprintf(", AllowErrors %v", e.AllowErrors)
	res, err := p.Edit(context.Background(), e)
	if err != nil {
		t.Errorf("API.md E19, %s, stable %v: %v", name, stable, err)
		return
	}
	if !slices.EqualFunc(res.Undo, want, sameOp) {
		t.Errorf("API.md E23, %s, stable %v: Undo %+v, want %+v", name, stable, res.Undo, want)
	}
	lock := lockOf(t, m, "p/canon.lock")
	if _, err := p.Edit(context.Background(), canon.Edit{Base: res.Revision, AllowErrors: true, Ops: res.Undo}); err != nil {
		t.Errorf("API.md E22, %s, stable %v: the Undo: %v", name, stable, err)
		return
	}
	if got := read(t, m, "p/p.canon"); got != broken {
		t.Errorf("API.md E22, %s, stable %v: source after the Undo:\n%s\nwant\n%s", name, stable, got, broken)
	}
	if got := lockOf(t, m, "p/canon.lock"); got != lock {
		t.Errorf("API.md E23, %s, stable %v: lock after the Undo %q, want %q", name, stable, got, lock)
	}
}
