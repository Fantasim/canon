package wire_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/wire"
)

// absentItem is the record of the DECISIONS 327 repro, one field per rule, and the host that
// evaluates its defaults.
func absentItem() (*types.RecordType, *host) {
	h := newHost()
	cost := field("dwCost", opt(types.IntType))
	cost.NoneWire = []byte(`"="`)
	noneDefault := h.withDefault(field("noneDefault", opt(types.IntType)), none(opt(types.IntType)))
	noneDefault.Default = &syntax.NoneLit{}
	someDefault := h.withDefault(field("someDefault", opt(types.IntType)), num(5))
	frame := h.withDefault(field("fFrame", types.FloatType), flt(1))
	item := record("p", "Item", field("dwID", types.StringType), field("dwNum", opt(types.IntType)),
		cost, noneDefault, someDefault, frame)
	return item, h
}

func absentRow(item *types.RecordType, id string, rest ...value.Value) *value.Record {
	return rec(item, append([]value.Value{str(id)}, rest...)...)
}

// WIRE.md §8.5, DECISIONS 327: an absent field is left out, and `load` gives the value back.
func TestTextAbsentField(t *testing.T) {
	item, h := absentItem()
	noInt := none(opt(types.IntType))
	for _, tc := range []struct {
		name string
		v    value.Value
		want string
	}{
		{"absent T? and = none omitted, marker kept, T? = d null", absentRow(item, "II_A", noInt, noInt, noInt, noInt, flt(1)),
			"{\n  \"dwID\": \"II_A\",\n  \"dwCost\": \"=\",\n  \"someDefault\": null,\n  \"fFrame\": 1\n}\n"},
		{"present values written", absentRow(item, "II_B", num(2), num(5), num(3), num(4), flt(2.5)),
			"{\n  \"dwID\": \"II_B\",\n  \"dwNum\": 2,\n  \"dwCost\": 5,\n  \"noneDefault\": 3,\n  \"someDefault\": 4,\n  \"fFrame\": 2.5\n}\n"},
	} {
		if got := text(t, tc.v); got != tc.want {
			t.Errorf("%s:\n%q\nwant:\n%q", tc.name, got, tc.want)
		}
		b, err := wire.Text(tc.v)
		if err != nil {
			t.Fatal(err)
		}
		got := decodeJSON(t, wire.Decoder{Host: h}, string(b), item)
		if !got.ok || !sameValue(got.v, tc.v) {
			t.Errorf("%s: round trip %s from %s", tc.name, got.text(), b)
		}
	}
}

// WIRE.md §8.5, DECISIONS 327: none as a list element, a map value or the whole value is null.
func TestTextAbsentElsewhere(t *testing.T) {
	noInt := none(opt(types.IntType))
	maybe := &types.MapType{Key: types.StringType, Value: opt(types.IntType)}
	listed := list(opt(types.IntType), num(1), noInt)
	mapped := &value.Map{T: maybe, Keys: []value.Value{str("a"), str("b")}, Vals: []value.Value{noInt, num(2)}}
	for _, tc := range []struct {
		name string
		v    value.Value
		typ  types.Type
		want string
	}{
		{"list element", listed, listOf(opt(types.IntType)), "[\n  1,\n  null\n]\n"},
		{"map value", mapped, maybe, "{\n  \"a\": null,\n  \"b\": 2\n}\n"},
		{"whole value", noInt, opt(types.IntType), "null\n"},
	} {
		if got := text(t, tc.v); got != tc.want {
			t.Errorf("%s: %q want %q", tc.name, got, tc.want)
		}
		roundTripText(t, tc.v, tc.typ)
	}
}

// WIRE.md §8.5, DECISIONS 327: a variant payload field and a record in a list follow the same rule.
func TestTextAbsentNested(t *testing.T) {
	item := &types.CaseType{Name: "item", Fields: []*types.Field{field("id", types.StringType), field("note", opt(types.StringType))}}
	gold := &types.CaseType{Name: "gold", Fields: []*types.Field{field("amount", types.IntType)}}
	vt := variantOf("shop", "Reward", "kind", item, gold)
	noNote := none(opt(types.StringType))
	reward := rec(item, str("sword"), noNote)
	if got, want := text(t, reward), "{\n  \"kind\": \"item\",\n  \"id\": \"sword\"\n}\n"; got != want {
		t.Errorf("variant: %q want %q", got, want)
	}
	roundTripText(t, reward, vt)
	badge := record("shop", "Badge", field("title", types.StringType), field("note", opt(types.StringType)))
	rows := list(badge, rec(badge, str("a"), noNote), rec(badge, str("b"), str("n")))
	if got, want := text(t, rows), "[\n  {\n    \"title\": \"a\"\n  },\n  {\n    \"title\": \"b\",\n    \"note\": \"n\"\n  }\n]\n"; got != want {
		t.Errorf("list of records: %q want %q", got, want)
	}
	roundTripText(t, rows, listOf(badge))
}
