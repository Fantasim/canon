package depsbaked_test

import (
	"testing"

	deps "example.com/data/depsbaked/out/go"
)

// CODEGEN.md §5.6: each baked value is in the branch its record's discriminant selects, and
// As<Branch> returns it with the branch's own Go type.
func TestDependentBaked(t *testing.T) {
	things := deps.GetThings()
	a, _ := things.Find("a")
	if v, ok := a.Ps().At(0).AsOne(); !ok || v != 1 {
		t.Errorf("a.Ps()[0].AsOne() = %d, %v, want 1, true", v, ok)
	}
	if v, ok := a.Q().AsTrue(); !ok || v != "yes" {
		t.Errorf("a.Q().AsTrue() = %q, %v, want yes, true", v, ok)
	}
	if v, ok := a.R().AsOne(); !ok || v != int32(7) || a.R().Branch() != deps.RBranchOne {
		t.Errorf("a.R().AsOne() = %d, %v, want 7, true", v, ok)
	}
	if v, ok := a.S().AsOne(); !ok || v != 9 {
		t.Errorf("a.S().AsOne() = %d, %v, want 9, true", v, ok)
	}
	b, _ := things.Find("b")
	if v, ok := b.Ps().At(0).AsTwo(); !ok || v != "p" {
		t.Errorf("b.Ps()[0].AsTwo() = %q, %v, want p, true", v, ok)
	}
	if v, ok := b.Q().AsFalse(); !ok || v != 2.5 || b.Q().Branch() != deps.QBranchFalse {
		t.Errorf("b.Q().AsFalse() = %v, %v, want 2.5, true", v, ok)
	}
	if v, ok := b.R().AsTwo(); !ok || v != "y" {
		t.Errorf("b.R().AsTwo() = %q, %v, want y, true", v, ok)
	}
	if b.S() != nil {
		t.Errorf("b.S() = %v, want nil", b.S())
	}
}
