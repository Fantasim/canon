package dep_test

import (
	"strings"
	"testing"

	dep "example.com/data/dep/out/go"
)

// CODEGEN.md §5.6, §5.8, DECISIONS 298: a define branch's key and value, looked up as it is read.
func TestDependentDefineDecode(t *testing.T) {
	things, err := dep.LoadThings("testdata/good/things.json")
	if err != nil {
		t.Fatal(err)
	}
	b, _ := things.Find("b")
	if k, is := b.P().AsTwo(); !is || k != "MI_B" {
		t.Errorf("b.P().AsTwo() = %q, %v", k, is)
	}
	if v, is := b.P().AsTwoValue(); !is || v != 9 {
		t.Errorf("b.P().AsTwoValue() = %d, %v, want 9, true", v, is)
	}
	a, _ := things.Find("a")
	if _, is := a.P().AsTwoValue(); is {
		t.Error("a.P().AsTwoValue() should be false: a's branch is one")
	}
}

// CODEGEN.md §5.8: a define the program's table lacks is the load error at the field.
func TestDependentDefineMissing(t *testing.T) {
	_, err := dep.LoadThings("testdata/bad/things.json")
	if err == nil || !strings.Contains(err.Error(), "rows[0].p: define MI_Z is not in this program's Monsters table") {
		t.Errorf("got %v, want the missing-define error at rows[0].p", err)
	}
}
