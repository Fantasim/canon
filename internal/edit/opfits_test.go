package edit_test

import (
	"context"
	"errors"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// fitsPaths are a target of each shape: lists and their elements, maps, tables and their
// entries, records, fields with and without a default, a case value.
var fitsPaths = []string{
	"a:items", "a:items[a]", "a:items[a].n", "a:byColor", "a:byColor[red]", "a:entries", "a:entries[one]",
	"a:entries[one].v", "a:shape", "a:shape.r", "a:base", "a:base.x", "a:bag.xs", "a:bag.xs[0]", "a:perEntry[one]",
}

// depPaths are dependent shapes: lists and maps of a type a field computes, a record typed by an
// argument, a variant case, a JSON root and its items; and in law a map keyed by a collection.
var depPaths = []string{
	"d:quests.gather.rewards", "d:quests.gather.rewards[0]", "d:quests.trade.prices", `d:quests.trade.prices["x"]`,
	"d:quests.raid.loot", "d:quests.raid.loot[0]", "d:quests.stash.pack", "d:quests.stash.pack.rewards[0]",
	"d:quests.hunt.act", "d:quests.hunt.act.rewards", "d:quests.hunt.act.rewards[0]", "d:loaded", "d:loaded.rewards[0]",
	`d:loaded.prices["x"]`, "d:quests.gather",
}

// lockedPaths are a stable table, an entry of it the lock holds, one it does not, and a plain table's.
var lockedPaths = []string{"p:ids", "p:ids[one]", "p:ids[two]", "p:rows[x]", "p:ids[one].label"}

// API.md 8.3, E4 (DECISIONS 462): fitters agree with the handlers: an op
// fits its target exactly where Apply, without an edit layer, refuses it with neither ErrBadOp nor
// ErrStableKey; that is the check behind the way out "make the edit without an edit layer".
func TestFitsAgreesWithApply(t *testing.T) {
	lockedFS := mapFS{
		"law/project.canon": file("project acme {\n  canon: \"0.1\"\n}\n"), "law/p/p.canon": file(lockedLaw),
		"law/p/canon.lock": file("# canon.lock v1\nfield  p.ids.code  1  one\ntable  p.ids  one\n"),
	}
	seen := map[bool]int{}
	checkFits(t, open(t, law, nil, "", "a"), fitsPaths, seen)
	checkFits(t, open(t, lockedFS, nil, "", "p"), lockedPaths, seen)
	checkFits(t, open(t, lootFS(), nil, "", "d"), depPaths, seen)
	checkFits(t, open(t, law, nil, "", "a"), []string{"a:perEntry", "a:picked.w", "a:picked.w[red]", "a:holder.data", "a:sq"}, seen)
	if seen[true] == 0 || seen[false] == 0 {
		t.Errorf("ops that fit %d, that do not %d: want both", seen[true], seen[false])
	}
}

// checkFits checks every op of parityOps on each path of s, counting them by fit in seen.
func checkFits(t *testing.T, s session, paths []string, seen map[bool]int) {
	t.Helper()
	f := fixture{Snapshot: s.snap, files: s.a.Files()}
	for _, path := range paths {
		r := resolve(t, f, path)
		for _, op := range parityOps(path, edit.Int(6)) {
			_, err := edit.Apply(context.Background(), s.env, s.snap, edit.Request{Ops: []edit.Operation{op}})
			if errors.Is(err, edit.ErrNotEditable) {
				continue // refused before its handler: the rows judge it
			}
			bad := errors.Is(err, edit.ErrBadOp) || errors.Is(err, edit.ErrStableKey)
			got := edit.Fits(s.snap, r, op.Kind)
			seen[got]++
			if got == bad {
				t.Errorf("%s %s: Fits %v, Apply %v", parityOpNames[op.Kind], path, got, err)
			}
		}
	}
}
