package value_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/value"
)

// lookupEqual is Lookup's equal: value equality, never aborting.
func lookupEqual(a, b value.Value) (bool, bool) { return value.Equal(a, b), true }

// DECISIONS 199, ADR-0018: a value Set replaces after Hash hashes and compares as built whole.
func TestMapSetReplaceAfterHash(t *testing.T) {
	m := pairs(str("a"), num(1), str("b"), num(2))
	before := value.Hash(m)
	m.Set(0, nil, num(9))
	want := pairs(str("a"), num(9), str("b"), num(2))
	if value.Hash(m) != value.Hash(want) || value.Hash(m) == before {
		t.Errorf("hash after Set: %x, want %x (was %x)", value.Hash(m), value.Hash(want), before)
	}
	if !value.Equal(m, want) || value.Equal(m, pairs(str("a"), num(1), str("b"), num(2))) {
		t.Errorf("equality after Set: %s", m.CanonText())
	}
	if v, ok := m.Get(str("a")); !ok || !value.Equal(v, num(9)) {
		t.Errorf("Get(a) after Set = %v, %v", v, ok)
	}
}

// DECISIONS 199, ADR-0018: keys Set appends after Lookup are found, hashed and compared.
func TestMapSetAppendAfterLookup(t *testing.T) {
	m := pairs(str("a"), num(1))
	if i, _ := m.Lookup(str("b"), lookupEqual); i != -1 {
		t.Fatalf("Lookup(b) before Set = %d", i)
	}
	_ = value.Hash(m)
	m.Set(-1, str("b"), num(2))
	m.Set(-1, str("c"), num(3))
	for i, k := range []string{"a", "b", "c"} {
		if j, ok := m.Lookup(str(k), lookupEqual); !ok || j != i {
			t.Errorf("Lookup(%s) after Set = %d, %v; want %d", k, j, ok, i)
		}
	}
	want := pairs(str("c"), num(3), str("a"), num(1), str("b"), num(2))
	if !value.Equal(m, want) || value.Hash(m) != value.Hash(want) {
		t.Errorf("after appends: %s, hash %x, want %s, hash %x", m.CanonText(), value.Hash(m), want.CanonText(), value.Hash(want))
	}
}
