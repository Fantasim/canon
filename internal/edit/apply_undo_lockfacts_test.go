package edit_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/value"
)

// API.md E23, E22, E20 (G2 round 4): each request locks hunt, new; the Undo applies, ends with
// hunt's Retire, keeps hunt as the edit's result holds it, and gives every other value back.
func TestUndoKeepsLockFacts(t *testing.T) {
	src := strings.Replace(questSrc, "let quests: table Quest", "let quests: stable table Quest", 1)
	collect := edit.Member("collect")
	add := edit.Operation{Kind: edit.OpAddEntry, Path: "d:quests", Key: edit.Key("hunt"), Value: edit.Source(`{ goal: kill, target: "elk", act: wait {} }`)}
	whole := func(hunt string) edit.Operation {
		return setAt("d:quests", edit.Source(`{ slay { goal: collect, target: 3, act: wait { turns: 2 } }, `+
			`gather { goal: collect, target: 10, act: hunt { goal: kill, target: "boar" } }`+hunt+` }`))
	}
	slay, huntGoal := setAt("d:quests.slay.goal", collect), setAt("d:quests.hunt.goal", collect)
	for _, ops := range [][]edit.Operation{
		{add, huntGoal, whole(`, hunt { goal: kill, target: "deer", act: wait {} }`)},
		{slay, add, huntGoal, whole(""), add},
		{slay, add, huntGoal, whole(""), add, setAt("d:quests.hunt.target", edit.Str("deer"))},
	} {
		fs := srcFS(src)()
		fs["law/d/canon.lock"] = file("# canon.lock v1\ntable  d.quests  gather\ntable  d.quests  slay\n")
		plan, f1, ok := applyIn(t, "lock facts", fs, undoView{}, ops)
		if !ok {
			continue
		}
		last := plan.Undo[len(plan.Undo)-1]
		if last.Kind != edit.OpRetire || last.Path != "d:quests.hunt" {
			t.Errorf("API.md E23, %+v: Undo %+v, want it to end with the Retire of d:quests.hunt", ops, plan.Undo)
		}
		back, f2, ok := applyIn(t, "lock facts, Undo", f1, undoView{}, plan.Undo)
		if !ok {
			continue
		}
		if want := []edit.Locked{{Name: "d.quests", Key: "hunt"}}; !slices.Equal(back.Locked, want) {
			t.Errorf("API.md E20, %+v: the Undo locks %+v, want %+v", ops, back.Locked, want)
		}
		w, g, after := valuesBut(t, fs, "hunt"), valuesBut(t, f2, "hunt"), entryText(t, f1, "hunt", false)
		for k, v := range w { //canon:unordered each value compared alone
			if g[k] != v {
				t.Errorf("API.md E22, %+v: %s = %s, want %s", ops, k, g[k], v)
			}
		}
		if got := entryText(t, f2, "hunt", true); after == "" || got != after {
			t.Errorf("API.md E23, %+v: hunt after the Undo %s, want %s retired", ops, got, after)
		}
	}
}

// valuesBut are the values of fsys's package d, d:quests without its entry key.
func valuesBut(t *testing.T, fsys mapFS, key string) map[string]string {
	t.Helper()
	s := open(t, fsys, nil, "", "d")
	out := values(s, false)
	if v, ok := s.a.Force(eval.Root{Pkg: "d", Name: "quests"}); ok {
		tbl := *v.(*value.Table)
		tbl.Entries = slices.DeleteFunc(slices.Clone(tbl.Entries), func(e *value.Record) bool { return e.Ident.Key.Text() == key })
		out["d:quests"] = tbl.CanonText()
	}
	return out
}

// entryText is the entry key of d:quests in fsys, and whether it is retired as retired wants.
func entryText(t *testing.T, fsys mapFS, key string, retired bool) string {
	t.Helper()
	v, ok := open(t, fsys, nil, "", "d").a.Force(eval.Root{Pkg: "d", Name: "quests"})
	if !ok {
		return ""
	}
	for _, e := range v.(*value.Table).Entries {
		if e.Ident.Key.Text() == key && e.Ident.Retired == retired {
			return e.CanonText()
		}
	}
	return ""
}
