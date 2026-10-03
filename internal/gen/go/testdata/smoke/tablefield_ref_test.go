package gentableref_test

import (
	"testing"

	p "example.com/data/gentableref/out/go"
)

// CODEGEN.md §5.8: a ref in a row of a nested table resolves to the entry of the holder's table.
func TestNestedRefs(t *testing.T) {
	shelves, err := p.LoadShelves("testdata/good/shelves.json")
	if err != nil {
		t.Fatal(err)
	}
	s1, _ := shelves.Find("s1")
	s2, _ := shelves.Find("s2")
	b, ok := s1.Slots().Find("b")
	if !ok || b.Home() != s2 {
		t.Errorf("b.Home() = %v, %v", b.Home(), ok)
	}
	if a, _ := s1.Slots().Find("a"); a.Home() != s1 || a.HomeID() != "s1" {
		t.Error("a.Home()")
	}
}

// A ref to no entry fails the load at the row of the nested table, named by its id.
func TestNestedRefMissing(t *testing.T) {
	_, err := p.LoadShelves("testdata/bad/home.json")
	want := "testdata/bad/home.json: rows[0].slots.a.home: no entry nowhere"
	if err == nil || err.Error() != want {
		t.Fatalf("%v, want %s", err, want)
	}
}
