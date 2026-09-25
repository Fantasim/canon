package genunionbaked_test

import (
	"testing"

	p "example.com/data/genunionbaked/out/go"
)

// CODEGEN.md §4.1, WIRE.md §5.9: a literal union getter returns the wire text of whichever arm
// matched, the checker's enum or the literal.
func TestLiteralUnion(t *testing.T) {
	items := p.GetItems()
	a, ok := items.Find("a")
	if !ok || a.Status() != "open" {
		t.Errorf("a.Status() = %q, %v, want %q, true", a.Status(), ok, "open")
	}
	b, ok := items.Find("b")
	if !ok || b.Status() != "unknown" {
		t.Errorf("b.Status() = %q, %v, want %q, true", b.Status(), ok, "unknown")
	}
}
