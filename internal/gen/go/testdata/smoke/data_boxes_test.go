package boxes_test

import (
	"testing"

	boxes "example.com/data/boxes/out/go"
	base "example.com/data/demo/base/out/go"
)

// A record field whose type is a keyed list (CODEGEN.md §4.2's nested path, not a top-level
// table or keyed-list value) decodes and keys its rows.
func TestBoxGood(t *testing.T) {
	b, err := boxes.LoadBox("testdata/good/box.json")
	if err != nil {
		t.Fatal(err)
	}
	if b.Items().Len() != 2 {
		t.Fatalf("items: %d", b.Items().Len())
	}
	if it, ok := b.Items().Find(1); !ok || it.A() != 1 {
		t.Error("find 1")
	}
}

// WIRE.md §5.7, TYPES.md §9.1's E3102 at run time: a nested keyed list refuses a duplicate key
// too, naming it as a decimal (its key field is Int) and where it was first seen; the key's own
// full wire path names it (`@json(path: "legacy.a")` here), as every other error on that field.
func TestBoxDup(t *testing.T) {
	_, err := boxes.LoadBox("testdata/bad/dup.json")
	want := "testdata/bad/dup.json: value.items[2].legacy.a: duplicate id 1 (first at value.items[0])"
	if err == nil || err.Error() != want {
		t.Fatalf("%v, want %s", err, want)
	}
}

// A nested keyed list whose key field is an enum decodes, and rejects a duplicate naming the
// key by its wire value, quoted (WIRE.md §5.3, §7.3).
func TestCrate(t *testing.T) {
	c, err := boxes.LoadCrate("testdata/good/crate.json")
	if err != nil {
		t.Fatal(err)
	}
	if s, ok := c.Slots().Find(boxes.SizeSmall); !ok || s.Size() != boxes.SizeSmall {
		t.Error("find small")
	}
	_, err = boxes.LoadCrate("testdata/bad/dupcrate.json")
	want := "testdata/bad/dupcrate.json: value.slots[2].size: duplicate id \"small\" (first at value.slots[0])"
	if err == nil || err.Error() != want {
		t.Fatalf("%v, want %s", err, want)
	}
}

// A nested keyed list whose key field is a @json(codes) enum decodes, and rejects a duplicate
// naming the key by its code, an unquoted decimal (WIRE.md §5.3, §7.3).
func TestVault(t *testing.T) {
	v, err := boxes.LoadVault("testdata/good/vault.json")
	if err != nil {
		t.Fatal(err)
	}
	if b, ok := v.Bins().Find(boxes.GradeBronze); !ok || b.Grade() != boxes.GradeBronze {
		t.Error("find bronze")
	}
	_, err = boxes.LoadVault("testdata/bad/dupvault.json")
	want := "testdata/bad/dupvault.json: value.bins[2].grade: duplicate id 1 (first at value.bins[0])"
	if err == nil || err.Error() != want {
		t.Fatalf("%v, want %s", err, want)
	}
}

// A nested keyed list whose key field is a ref (TYPES.md §9.1's keyable() accepts one too)
// decodes and keys by the ref's own key (here a String), and rejects a duplicate the same way.
func TestTicketBox(t *testing.T) {
	tb, err := boxes.LoadTicketBox("testdata/good/ticketbox.json")
	if err != nil {
		t.Fatal(err)
	}
	if ti, ok := tb.Tickets().Find("a"); !ok || ti.LabelID() != "a" {
		t.Error("find a")
	}
	_, err = boxes.LoadTicketBox("testdata/bad/dupticketbox.json")
	want := "testdata/bad/dupticketbox.json: value.tickets[2].label: duplicate id \"a\" (first at value.tickets[0])"
	if err == nil || err.Error() != want {
		t.Fatalf("%v, want %s", err, want)
	}
}

// A nested keyed list whose key field is a ref into an imported baked table with enum ids: the
// duplicate names the key by that enum's String() (the table key), never Wire() (it has none).
func TestTrophy(t *testing.T) {
	tr, err := boxes.LoadTrophy("testdata/good/trophy.json")
	if err != nil {
		t.Fatal(err)
	}
	if a, ok := tr.Awards().Find(base.RankIDLow); !ok || a.RankID() != base.RankIDLow {
		t.Error("find low")
	}
	_, err = boxes.LoadTrophy("testdata/bad/duptrophy.json")
	want := "testdata/bad/duptrophy.json: value.awards[2].rank: duplicate id \"low\" (first at value.awards[0])"
	if err == nil || err.Error() != want {
		t.Fatalf("%v, want %s", err, want)
	}
}
