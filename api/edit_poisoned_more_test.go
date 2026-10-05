package canon_test

import (
	"context"
	"errors"
	"slices"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// twoBroken is package p with two entries missing hp: the table stays poisoned whatever one
// operation repairs; lvl has a default.
const twoBroken = "package p\n\nrecord M {\n  hp: Int\n  lvl: Int = 1\n}\n\nlet ms: table M = {\n  wolf { hp: 1 }\n  bat {}\n  cat {}\n}\n"

// API.md E19, E23, E22, DECISIONS 309: on a table that stays poisoned, each repair and its Undo
// work from the sources; without AllowErrors the repair is rejected, the table still in error.
func TestEditPoisonedStaysPoisoned(t *testing.T) {
	cases := []struct {
		name string
		op   canon.Op
		want []canon.Op
	}{
		{"Remove", canon.Remove("p:ms.bat"), []canon.Op{canon.AddEntry("p:ms", canon.Key("bat"), canon.Source("{}")),
			canon.Move("p:ms.bat", 1)}},
		{"Remove the first", canon.Remove("p:ms.wolf"), []canon.Op{canon.AddEntry("p:ms", canon.Key("wolf"), canon.Source("{ hp: 1 }")),
			canon.Move("p:ms.wolf", 0)}},
		{"Set a required field", canon.Set("p:ms.bat.hp", canon.Int(3)), []canon.Op{canon.Set("p:ms.bat", canon.Source("{}"))}},
		{"Set a defaulted field", canon.Set("p:ms.bat.lvl", canon.Int(2)), []canon.Op{canon.Reset("p:ms.bat.lvl")}},
		{"Move", canon.Move("p:ms.cat", 0), []canon.Op{canon.Move("p:ms.cat", 2)}},
	}
	for _, c := range cases {
		stillPoisoned(t, c.name, c.op, c.want)
	}
}

// stillPoisoned applies op to twoBroken, then its Undo, as TestEditPoisonedStaysPoisoned says.
func stillPoisoned(t *testing.T, name string, op canon.Op, want []canon.Op) {
	t.Helper()
	ctx := context.Background()
	p, m := openEdit(t, map[string]string{"p/p.canon": twoBroken})
	if _, err := p.Edit(ctx, canon.Edit{Ops: []canon.Op{op}}); !errors.Is(err, canon.ErrRejected) {
		t.Errorf("API.md E19, %s without AllowErrors: %v, want ErrRejected", name, err)
	}
	res, err := p.Edit(ctx, canon.Edit{AllowErrors: true, Ops: []canon.Op{op}})
	if err != nil {
		t.Errorf("API.md E19, %s: %v", name, err)
		return
	}
	if !slices.EqualFunc(res.Undo, want, sameOp) {
		t.Errorf("API.md E23, %s: Undo %+v, want %+v", name, res.Undo, want)
	}
	if _, err := p.Edit(ctx, canon.Edit{Base: res.Revision, AllowErrors: true, Ops: res.Undo}); err != nil {
		t.Errorf("API.md E22, %s: the Undo %+v: %v", name, res.Undo, err)
		return
	}
	if got := read(t, m, "p/p.canon"); got != twoBroken {
		t.Errorf("API.md E22, %s: source after the Undo:\n%s", name, got)
	}
}

// API.md E4, E1, DECISIONS 309: on a poisoned stable table a held entry stays unremovable, a
// missing one is ErrNoPath, and an operation the sources alone do not decide stays ErrNoValue.
func TestEditPoisonedRefused(t *testing.T) {
	cases := []struct {
		op   canon.Op
		want error
	}{
		{canon.Remove("p:ms.wolf"), canon.ErrStableKey},
		{canon.Remove("p:ms.owl"), canon.ErrNoPath},
		{canon.Rename("p:ms.bat", canon.Key("owl")), canon.ErrNoValue},
		{canon.Set("p:ms", canon.Source(`{ wolf { hp: 1 } }`)), canon.ErrNoValue},
	}
	for _, c := range cases {
		p, _, _ := poisoned(t, true)
		if _, err := p.Edit(context.Background(), canon.Edit{AllowErrors: true, Ops: []canon.Op{c.op}}); !errors.Is(err, c.want) {
			t.Errorf("API.md E1, E4, %+v: %v, want %v", c.op, err, c.want)
		}
	}
}
