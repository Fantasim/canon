package gennestunion_test

import (
	"testing"

	p "example.com/data/gennestunion/out/go"
)

// CODEGEN.md §4.1, DECISIONS 387, 456: a union getter returns the wire text of the ref arm's key, down a
// chain of refs, or the literal.
func TestNestedRefUnion(t *testing.T) {
	items := p.GetItems()
	a, ok := items.Find("a")
	if !ok || a.Pick() != "z2" {
		t.Errorf("a.Pick() = %q, %v, want %q, true", a.Pick(), ok, "z2")
	}
	b, ok := items.Find("b")
	if !ok || b.Pick() != "none" {
		t.Errorf("b.Pick() = %q, %v, want %q, true", b.Pick(), ok, "none")
	}
}
