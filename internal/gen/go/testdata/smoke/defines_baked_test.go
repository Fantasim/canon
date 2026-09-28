package defsbaked_test

import (
	"slices"
	"testing"

	defs "example.com/data/defsbaked/out/go"
)

// CODEGEN.md §5.8: each define ref's value is baked beside its key, for one, an optional, a list
// and an optional list.
func TestDefinesBaked(t *testing.T) {
	hits := defs.GetHits()
	a, _ := hits.Find("a")
	if a.MonsterID() != "MI_A" || a.MonsterValue() != 7 {
		t.Errorf("a.Monster = %q %d, want MI_A 7", a.MonsterID(), a.MonsterValue())
	}
	if v, ok := a.AltValue(); !ok || v != 9 {
		t.Errorf("a.AltValue() = %d, %v, want 9, true", v, ok)
	}
	if got := slices.Collect(a.AllValues().All()); !slices.Equal(got, []int64{7, 9}) {
		t.Errorf("a.AllValues() = %v, want [7 9]", got)
	}
	if _, ok := a.MaybeValues(); ok {
		t.Error("a.MaybeValues() is set, want none")
	}
	b, _ := hits.Find("b")
	if _, ok := b.AltValue(); ok {
		t.Error("b.AltValue() is set, want none")
	}
	if v, ok := b.MaybeValues(); !ok || v.Len() != 1 || v.At(0) != 7 {
		t.Errorf("b.MaybeValues() = %v, %v, want [7], true", v, ok)
	}
}
