package gentablebaked_test

import (
	"testing"

	p "example.com/data/gentablebaked/out/go"
)

// CODEGEN.md §4.2, §5.3: a table field is an rt.KeyedList of the rows by id, in entry order, and a retired entry keeps its row.
func TestTableField(t *testing.T) {
	s := p.GetShelf()
	slots := s.Slots()
	if slots.Len() != 2 {
		t.Fatalf("slots: %d rows, want 2", slots.Len())
	}
	a, ok := slots.Find("a")
	if !ok || a.N() != 1 || a.ID() != "a" || a.Retired() {
		t.Errorf("a = %v, %v", a, ok)
	}
	b, ok := slots.Find("b")
	if !ok || b.N() != 2 || !b.Retired() {
		t.Errorf("b = %v, %v", b, ok)
	}
	if slots.At(0) != a || slots.At(1) != b {
		t.Error("rows are not in entry order")
	}
	spare, ok := s.Spare()
	if c, found := spare.Find("c"); !ok || !found || c.N() != 3 {
		t.Errorf("spare = %v, %v", c, found)
	}
}

// CODEGEN.md §5.8, DECISIONS 288: a ref into a table field is keyed by the field's id type, so Find takes it as is.
func TestTableFieldRef(t *testing.T) {
	s := p.GetShelf()
	if fav, ok := s.Slots().Find(s.FavID()); !ok || fav.N() != 2 {
		t.Errorf("fav = %v, %v", fav, ok)
	}
	next, ok := s.NextID()
	if got, found := s.Slots().Find(next); !ok || !found || got.N() != 1 {
		t.Errorf("next = %v, %v", got, found)
	}
}
