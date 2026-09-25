package dep_test

import (
	"strings"
	"testing"

	dep "example.com/data/dep/out/go"
)

// CODEGEN.md §5.6: an untagged dependent value decoded from the discriminant an earlier field
// of the same record already holds (ArgField, no further path).
func TestDependentDecode(t *testing.T) {
	things, err := dep.LoadThings("testdata/good/things.json")
	if err != nil {
		t.Fatal(err)
	}
	a, ok := things.Find("a")
	if !ok {
		t.Fatal("a not found")
	}
	if v, is := a.P().AsOne(); !is || v != 42 {
		t.Errorf("a.P().AsOne() = %d, %v, want 42, true", v, is)
	}
	if _, is := a.P().AsTwo(); is {
		t.Error("a.P().AsTwo() should be false: a's branch is one")
	}
	if a.P().Branch() != dep.PBranchOne {
		t.Errorf("a.P().Branch() = %v, want PBranchOne", a.P().Branch())
	}

	b, ok := things.Find("b")
	if !ok {
		t.Fatal("b not found")
	}
	if v, is := b.P().AsTwo(); !is || v != "hi" {
		t.Errorf("b.P().AsTwo() = %q, %v, want \"hi\", true", v, is)
	}
	if _, is := b.P().AsOne(); is {
		t.Error("b.P().AsOne() should be false: b's branch is two")
	}
}

// CODEGEN.md §5.6: a discriminant member no branch covers is a load error naming the field's
// own path, without a trailing dot (rows[0].p, not rows[0].p.).
func TestDependentUnknownCase(t *testing.T) {
	_, err := dep.LoadThings("testdata/bad/things.json")
	if err == nil {
		t.Fatal("want an error: k selects a branch P does not cover")
	}
	msg := err.Error()
	if !strings.Contains(msg, "rows[0].p: unknown case three") {
		t.Errorf("got %q, want it to name rows[0].p and the unknown case", msg)
	}
	if strings.Contains(msg, "rows[0].p.:") {
		t.Errorf("got %q, a trailing dot after the field path", msg)
	}
}
