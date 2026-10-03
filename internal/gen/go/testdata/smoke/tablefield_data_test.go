package gentabledata_test

import (
	"testing"

	p "example.com/data/gentabledata/out/go"
)

// WIRE.md §5.7: a nested table is an object keyed by entry id; its rows keep file order, a row's `$retired` is its retired flag.
func TestTableFieldGood(t *testing.T) {
	s, err := p.LoadShelf("testdata/good/shelf.json")
	if err != nil {
		t.Fatal(err)
	}
	slots := s.Slots()
	if slots.Len() != 2 || slots.At(0).ID() != "a" || slots.At(1).ID() != "b" {
		t.Fatalf("slots: %d rows", slots.Len())
	}
	if slots.At(0).Retired() || !slots.At(1).Retired() || slots.At(1).N() != 2 {
		t.Error("retired flags")
	}
	if b, ok := slots.Find("b"); !ok || b.N() != 2 {
		t.Error("find b")
	}
	spare, ok := s.Spare()
	if c, found := spare.Find("c"); !ok || !found || c.N() != 3 {
		t.Errorf("spare: %v, %v", ok, found)
	}
}

// An absent or null optional table is none; an empty object is an empty table.
func TestTableFieldNone(t *testing.T) {
	for _, file := range []string{"empty", "nullspare"} {
		s, err := p.LoadShelf("testdata/good/" + file + ".json")
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := s.Spare(); ok {
			t.Errorf("%s: spare is present", file)
		}
	}
	s, err := p.LoadShelf("testdata/good/empty.json")
	if err != nil || s.Slots().Len() != 0 {
		t.Errorf("empty: %v", err)
	}
}

// WIRE.md §5.7 (E7114, E7110): a key that is no identifier, a `$retired` other than true, a non-object, a bad row and a repeated key fail the load at their place.
func TestTableFieldBad(t *testing.T) {
	for file, want := range map[string]string{
		"key":     `testdata/bad/key.json: value.slots: "a b" is not a valid table key (an identifier)`,
		"retired": `testdata/bad/retired.json: value.slots.a.$retired: expected true`,
		"array":   `testdata/bad/array.json: value.slots: expected an object`,
		"row":     `testdata/bad/row.json: value.slots.b.n: expected an integer`,
		"dup":     `testdata/bad/dup.json: value.slots.a: duplicate key`,
	} {
		_, err := p.LoadShelf("testdata/bad/" + file + ".json")
		if err == nil || err.Error() != want {
			t.Errorf("%s: %v, want %s", file, err, want)
		}
	}
}

// DECISIONS 288: a ref into a table field reads as the field's id type.
func TestTableFieldRef(t *testing.T) {
	s, err := p.LoadShelf("testdata/good/shelf.json")
	if err != nil {
		t.Fatal(err)
	}
	if fav, ok := s.Slots().Find(s.FavID()); !ok || fav.N() != 1 {
		t.Errorf("fav = %v, %v", fav, ok)
	}
	next, ok := s.NextID()
	if got, found := s.Slots().Find(next); !ok || !found || got.N() != 2 {
		t.Errorf("next = %v, %v", got, found)
	}
}
