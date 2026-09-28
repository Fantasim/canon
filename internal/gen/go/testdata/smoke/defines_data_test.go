package defsdata_test

import (
	"slices"
	"testing"

	defs "example.com/data/defsdata/out/go"
)

// CODEGEN.md §5.8: the loader stores each define's value beside its key.
func TestDefinesLoaded(t *testing.T) {
	hits, err := defs.LoadHits("testdata/good/hits.json")
	if err != nil {
		t.Fatal(err)
	}
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
	b, _ := hits.Find("b")
	if v, ok := b.MaybeValues(); !ok || v.Len() != 1 || v.At(0) != 7 {
		t.Errorf("b.MaybeValues() = %v, %v, want [7], true", v, ok)
	}
}

// CODEGEN.md §5.8: a key missing from the table is the load error at its path.
func TestDefineMissing(t *testing.T) {
	_, err := defs.LoadHits("testdata/bad/hits.json")
	want := "testdata/bad/hits.json: rows[0].all[1]: define MI_C is not in this program's Monsters table"
	if err == nil || err.Error() != want {
		t.Fatalf("got %v, want %s", err, want)
	}
}
