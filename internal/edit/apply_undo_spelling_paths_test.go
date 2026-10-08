package edit_test

import (
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// spelledQuestFS is questSrc with its numbers and strings spelled other than canonically.
func spelledQuestFS() mapFS {
	src := strings.Replace(questSrc, `slay { goal: kill, target: "wolf", bonus: "bear", act: wait { turns: 2 }, rewards: [100, 50] }`,
		"slay {\n    goal: kill\n    target: r\"wolf\"\n    bonus: \"b\\u{65}ar\"\n    act: wait { turns: 2 }\n    rewards: [1_00, 0x32]\n  }", 1)
	src = strings.Replace(src, `gather { goal: collect, target: 10, act: hunt { goal: kill, target: "boar" } }`,
		`gather { goal: collect, target: 0x0A, act: hunt { goal: kill, target: r"boar" } }`, 1)
	return mapFS{"law/project.canon": file(projectCanon), "law/d/d.canon": file(src)}
}

// DECISIONS 337, 257, API.md E22: the verified Undo's region restores, the dependent values a
// Source types through a branch, and E15's drop give each token back as written.
func TestUndoSpellingRepairAndDependent(t *testing.T) {
	collect, kill := edit.Member("collect"), edit.Member("kill")
	for _, c := range []struct {
		name string
		ops  []edit.Operation
	}{
		{"driver then dependent", []edit.Operation{setAt("quests.slay.goal", collect), setAt("quests.slay.target", edit.Int(25))}},
		{"two dependents", []edit.Operation{
			setAt("quests.slay.goal", collect), setAt("quests.slay.target", edit.Int(25)), setAt("quests.slay.bonus", edit.Int(3)),
		}},
		{"dependent set twice", []edit.Operation{
			setAt("quests.slay.goal", collect), setAt("quests.slay.target", edit.Int(25)), setAt("quests.slay.target", edit.Int(30)),
		}},
		{"driver changed twice", []edit.Operation{
			setAt("quests.slay.goal", collect), setAt("quests.slay.target", edit.Int(25)),
			setAt("quests.slay.goal", kill), setAt("quests.slay.target", edit.Str("boar")),
		}},
		{"inside a case", []edit.Operation{setAt("quests.gather.act.goal", collect), setAt("quests.gather.act.target", edit.Int(5))}},
		{"list, Remove", []edit.Operation{setAt("quests.slay.items", edit.Bool(true)), {Kind: edit.OpRemove, Path: "quests.slay.rewards[0]"}}},
		{"list, whole", []edit.Operation{setAt("quests.slay.items", edit.Bool(true)), setAt("quests.slay.rewards", edit.Source(`["potion"]`))}},
		{"a dependent list's Source", []edit.Operation{setAt("quests.slay.rewards", edit.Source("[0x1, 2_0]"))}},
		{"E15 drops 0x0A", []edit.Operation{setAt("quests.gather.goal", kill)}},
	} {
		undoTwiceFiles(t, c.name, spelledQuestFS(), c.ops)
	}
}

// DECISIONS 337, API.md V1: a dependent value a Source gives is written as the Source spelled it.
func TestDependentSourceSpelling(t *testing.T) {
	_, f1, ok := applyIn(t, "dependent list", spelledQuestFS(), undoView{}, []edit.Operation{setAt("quests.slay.rewards", edit.Source("[0x1, 2_0]"))})
	if ok && !strings.Contains(string(f1["law/d/d.canon"].Data), "rewards: [0x1, 2_0]") {
		t.Errorf("the rewards are not written as the Source spelled them:\n%s", f1["law/d/d.canon"].Data)
	}
}

const movesSrc = `package d

/// A move.
record Move {
  /// Its multiplier.
  mul: Float
  /// Its mask.
  mask: Int
  /// Its label.
  label: String = ""
}

/// The moves.
let moves: stable table Move = {
  slash { mul: 1.0, mask: 0x1F }
  bash { mul: 2.50, mask: 1_000, label: r"x\y" }
}
`

// DECISIONS 337, API.md E20, E23: the whole write back of a stable table, its lock facts kept,
// gives the entries it restores their tokens as written.
func TestUndoHeldTableSpelling(t *testing.T) {
	fs := mapFS{"law/project.canon": file(projectCanon), "law/d/d.canon": file(movesSrc),
		"law/d/canon.lock": file("# canon.lock v1\ntable  d.moves  bash\ntable  d.moves  slash\n")}
	ops := []edit.Operation{
		{Kind: edit.OpAddEntry, Path: "d:moves", Key: edit.Key("kick"), Value: edit.Source(`{ mul: 3, mask: 3 }`)},
		setAt("d:moves", edit.Source(`{ slash { mul: 5, mask: 5 }, bash { mul: 6, mask: 6 }, kick { mul: 7, mask: 7 } }`)),
	}
	p, f1, ok := applyIn(t, "held", fs, undoView{}, ops)
	if !ok {
		return
	}
	_, f2, ok := applyIn(t, "held, Undo", f1, undoView{}, p.Undo)
	if !ok {
		return
	}
	got := string(f2["law/d/d.canon"].Data)
	if !strings.Contains(got, "slash { mul: 1.0, mask: 0x1F }") || !strings.Contains(got, `bash { mul: 2.50, mask: 1_000, label: r"x\y" }`) {
		t.Errorf("the Undo %+v does not give the entries back as written:\n%s", p.Undo, got)
	}
}

// DECISIONS 337, API.md E22: a map's keys, numbers or strings, come back as written too.
func TestUndoMapKeySpelling(t *testing.T) {
	for _, c := range []struct{ name, let, set string }{
		{"Int keys", "let m: {Int: Float} = { 0x1: 1.0, 2: 2.50 }", "{ 5: 5 }"},
		{"String keys", `let m: {String: String} = { r"a\b": r"c\d" }`, `{ "k": "v" }`},
	} {
		src := "package d\n\n/// A map.\n" + c.let + "\n"
		fs := mapFS{"law/project.canon": file(projectCanon), "law/d/d.canon": file(src)}
		undoTwiceFiles(t, c.name, fs, []edit.Operation{setAt("m", edit.Source(c.set))})
	}
}
