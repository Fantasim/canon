package dep_test

import (
	"testing"

	dep "example.com/data/dep/out/go"
)

// CODEGEN.md §5.6, DECISIONS 298: a baked define branch holds its key and its value.
func TestDependentDefineBaked(t *testing.T) {
	b := dep.GetThings().Get(dep.ThingIDB)
	if k, is := b.P().AsTwo(); !is || k != "MI_B" {
		t.Errorf("AsTwo() = %q, %v", k, is)
	}
	if v, is := b.P().AsTwoValue(); !is || v != 9 {
		t.Errorf("AsTwoValue() = %d, %v, want 9, true", v, is)
	}
	if _, is := dep.GetThings().Get(dep.ThingIDA).P().AsTwoValue(); is {
		t.Error("a's AsTwoValue() should be false")
	}
}
