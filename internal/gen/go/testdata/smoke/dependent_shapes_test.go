package deps_test

import (
	"strings"
	"testing"

	deps "example.com/data/deps/out/go"
)

// CODEGEN.md §5.6: a list of dependent values shares the field's discriminant, a Bool selects
// Q's branch, R's match reads ev.k through the record held by value, and s's argument is ev.k.
func TestDependentShapes(t *testing.T) {
	things, err := deps.LoadThings("testdata/good/things.json")
	if err != nil {
		t.Fatal(err)
	}
	a, _ := things.Find("a")
	if v, ok := a.Ps().At(1).AsOne(); !ok || v != 2 {
		t.Errorf("a.Ps()[1].AsOne() = %d, %v, want 2, true", v, ok)
	}
	if v, ok := a.Q().AsTrue(); !ok || v != "yes" || a.Q().Branch() != deps.QBranchTrue {
		t.Errorf("a.Q().AsTrue() = %q, %v, want yes, true", v, ok)
	}
	if v, ok := a.R().AsOne(); !ok || v != 7 {
		t.Errorf("a.R().AsOne() = %d, %v, want 7, true", v, ok)
	}
	if v, ok := a.S().AsOne(); !ok || v != 9 {
		t.Errorf("a.S().AsOne() = %d, %v, want 9, true", v, ok)
	}
	b, _ := things.Find("b")
	if v, ok := b.Ps().At(0).AsTwo(); !ok || v != "p" {
		t.Errorf("b.Ps()[0].AsTwo() = %q, %v, want p, true", v, ok)
	}
	if v, ok := b.Q().AsFalse(); !ok || v != 2.5 {
		t.Errorf("b.Q().AsFalse() = %v, %v, want 2.5, true", v, ok)
	}
	if v, ok := b.R().AsTwo(); !ok || v != "y" || b.R().Branch() != deps.RBranchTwo {
		t.Errorf("b.R().AsTwo() = %q, %v, want y, true (three is the wildcard arm's)", v, ok)
	}
	if b.S() != nil {
		t.Errorf("b.S() = %v, want nil: s is absent", b.S())
	}
}

// WIRE.md §5.9: a present value whose discriminant selects a Never arm is a load error at its own path.
func TestDependentNeverArm(t *testing.T) {
	_, err := deps.LoadThings("testdata/bad/things.json")
	if err == nil || !strings.Contains(err.Error(), "rows[0].s: unknown case three") {
		t.Fatalf("got %v, want the Never arm refused at rows[0].s", err)
	}
}
