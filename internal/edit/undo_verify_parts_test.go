package edit_test

import (
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// API.md E22 (log-2026-10-01 M4.1 rulings): an entry whose file is not where the Undo must leave
// it is a difference at the entry, whatever its value.
func TestUndoVerifiesPlacement(t *testing.T) {
	rt := &types.RecordType{Name: "R"}
	entry := func() *value.Record { return &value.Record{T: rt, Ident: &value.Identity{Key: value.Key{S: "a"}}} }
	tt := &types.TableType{}
	x := &value.Table{T: tt, Entries: []*value.Record{entry()}}
	y := &value.Table{T: tt, Entries: []*value.Record{entry()}}
	elsewhere := func(string, *value.Record, *value.Record) bool { return false }
	if got := edit.DiffPaths("d:t", x, y, true, elsewhere); !slices.Equal(got, []string{"d:t.a"}) {
		t.Errorf("an entry in another file: %v, want [d:t.a]", got)
	}
	if got := edit.DiffPaths("d:t", x, y, true, nil); len(got) != 0 {
		t.Errorf("an entry in its file: %v, want none", got)
	}
}

// API.md E22, E23 (Move, log-2026-09-29 U-E22-r): an Undo's verification counts a map or a
// source-ordered table whose keys come back in another order as not given back, at the
// collection, which a Move or its restore puts in order; one whose order its files give, not.
func TestUndoVerifiesKeyOrder(t *testing.T) {
	str := func(s string) value.Value { return &value.Str{V: s, T: types.StringType} }
	num := func(n int64) value.Value { return &value.Int{V: n, T: types.IntType} }
	mt := &types.MapType{Key: types.StringType, Value: types.IntType}
	before := &value.Map{T: mt, Keys: []value.Value{str("a"), str("b")}, Vals: []value.Value{num(1), num(2)}}
	swapped := &value.Map{T: mt, Keys: []value.Value{str("b"), str("a")}, Vals: []value.Value{num(2), num(1)}}
	if got := edit.DiffPaths("d:m", before, swapped, true, nil); !slices.Equal(got, []string{"d:m"}) {
		t.Errorf("keys in another order: %v, want [d:m]", got)
	}
	if got := edit.DiffPaths("d:m", before, swapped, false, nil); len(got) != 0 {
		t.Errorf("keys in another order, ordered by files: %v, want none", got)
	}
	changed := &value.Map{T: mt, Keys: []value.Value{str("a"), str("b")}, Vals: []value.Value{num(1), num(3)}}
	if got := edit.DiffPaths("d:m", before, changed, true, nil); !slices.Equal(got, []string{"d:m[b]"}) {
		t.Errorf("a value changed: %v, want [d:m[b]]", got)
	}
}

// refRootSrc holds a ref into quests in a root no operation names.
const refRootSrc = questSrc + `
/// A favourite quest.
let fav: ref quests = gather
`

// API.md E11, E22: a Rename rewrites references anywhere, so with one among the operations the
// Undo's verification compares every let of the packages written, a root no operation names
// included; without one, only the roots the operations name.
func TestUndoComparesEveryRootUnderRename(t *testing.T) {
	s := open(t, srcFS(refRootSrc)(), nil, "", "d")
	ops := []edit.Operation{setAt("d:quests.slay.goal", edit.Member("collect"))}
	if got := edit.RootsCompared(s.snap, ops, true, []string{"d"}); !slices.Contains(got, "d:fav") {
		t.Errorf("with a Rename: %v, want d:fav among them", got)
	}
	if got := edit.RootsCompared(s.snap, ops, false, []string{"d"}); slices.Contains(got, "d:fav") {
		t.Errorf("without a Rename: %v, want no d:fav", got)
	}
	undoIn(t, undoSpec{name: "rename with a ref beside", fsys: srcFS(refRootSrc)(), views: [][]string{nil}, ops: []edit.Operation{
		setAt("quests.gather.goal", edit.Member("kill")), {Kind: edit.OpRename, Path: "quests.gather", Key: edit.Key("pick")},
		setAt("quests.pick.target", edit.Str("x")),
	}})
}
