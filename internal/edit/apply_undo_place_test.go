package edit_test

import (
	"maps"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// tagFilesSrc names each entry's file by a field E15 drops when its driver changes.
const tagFilesSrc = `package d

/// What a quest counts.
enum Goal { kill, collect }

/// What a quest aims at, by goal.
type Target(g: Goal) = match g {
  kill => String(1..)
  collect => Int(1..)
}

/// A quest.
record Quest {
  /// What it counts.
  goal: Goal
  /// Its tag.
  tag: Target(goal)?
}

/// The quests, each in a file named by its tag.
@files("q/{tag}/{id}.canon")
let quests: table Quest = {}
`

// API.md E22, N1, N2, N3, N4 (log-2026-10-01 M4.1 rulings): an entry an Undo recreates may lie at
// any path its template gives its key, whatever the templated fields a later Set changes; the
// Undo applies, and so does the Undo of that Undo, both verified, every value coming back.
func TestUndoRecreatedAnyTemplatePath(t *testing.T) {
	collect, kill := edit.Member("collect"), edit.Member("kill")
	for _, ops := range [][]edit.Operation{
		{setAt("quests.gather.target", edit.Int(6)), {Kind: edit.OpRename, Path: "quests.slay", Key: edit.Key("tmp")},
			{Kind: edit.OpAddEntry, Path: "quests", Key: edit.Key("slay"), Value: edit.Source(`{ goal: collect, target: 3 }`)},
			setAt("quests.gather.goal", kill), setAt("quests.slay.goal", kill)},
		{setAt("quests.slay.goal", collect), {Kind: edit.OpRemove, Path: "quests.slay"}},
	} {
		undoIn(t, undoSpec{name: "recreated", fsys: filesFS(), ops: ops, views: [][]string{nil}})
	}
}

// API.md E11, E15, E22, N8: a renamed entry whose driver changes loses the field naming its file;
// E23's order renames it back before that field returns, so only verification sees the file stay,
// and restores the entry before the Rename, which moves it back (log-2026-10-01 M4.1 rulings).
func TestUndoFileFollowsTemplate(t *testing.T) {
	fs := mapFS{"law/project.canon": file(projectCanon), "law/d/d.canon": file(tagFilesSrc),
		"law/d/q/wolf/slay.canon": file("package d\n\n/// Slay.\nentry quests.slay { goal: kill, tag: \"wolf\" }\n")}
	ops := []edit.Operation{{Kind: edit.OpRename, Path: "quests.slay", Key: edit.Key("slain")}, setAt("quests.slain.goal", edit.Member("collect"))}
	plan, f1, ok := applyIn(t, "renamed, driver changed", fs, undoView{}, ops)
	if !ok {
		return
	}
	_, f2, ok := applyIn(t, "renamed, driver changed, Undo", f1, undoView{}, plan.Undo)
	if !ok {
		return
	}
	sameIn(t, "renamed, driver changed, Undo", fs, f2, nil)
	if w, g := slices.Sorted(maps.Keys(fs)), slices.Sorted(maps.Keys(f2)); !slices.Equal(w, g) {
		t.Errorf("the Undo leaves files %v, want %v", g, w)
	}
}
