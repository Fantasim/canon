package wire_test

import (
	"errors"
	"testing"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/wire"
)

// fpdemoSkill is the `fpdemo` package of WIRE.md §8.3 (FINGERPRINT.md vector 6).
func fpdemoSkill() (*types.RecordType, *types.EnumType, *types.EnumType) {
	element := codes(enum("fpdemo", "Element", "FIRE", "WATER", "WIND"), 1, 2, 3)
	element.WireCodes, element.Members[2].Retired = true, true
	flag := codes(enum("fpdemo", "Flag", "tradable", "droppable", "soulbound"), 1, 2, 4)
	flag.Codes = &types.UInt32Type
	side := enum("fpdemo", "Side", "left", "right")
	side.Members[1].Wire = "RIGHT"
	elementField := field("element", &types.OptionalType{Elem: element})
	elementField.NoneWire = []byte("0")
	flags := field("flags", &types.ListType{Elem: flag})
	flags.Enc = types.EncBits
	twoHanded := field("twoHanded", types.BoolType, "bTwoHanded")
	twoHanded.Enc = types.EncInt
	castTime := field("castTime", &types.OptionalType{Elem: types.DurationType}, "cast_time")
	castTime.Unit = types.UnitS
	skill := record("fpdemo", "Skill", field("id", types.StringType, "dwID"),
		field("reqMp", types.IntType, "legacy", "reqMp"), field("reqFp", types.IntType, "legacy", "reqFp"),
		elementField, flags, twoHanded, field("side", &types.LitUnionType{Of: side, Literals: []string{"both"}}),
		castTime, field("weights", &types.MapType{Key: element, Value: types.FloatType}))
	return skill, element, flag
}

// WIRE.md §8.3: every legacy encoding in one row.
func TestSampleLegacyRow(t *testing.T) {
	skill, element, flag := fpdemoSkill()
	weights := &value.Map{T: skill.Fields[8].Type, Keys: []value.Value{member(element, 0), member(element, 1)}, Vals: []value.Value{flt(0.5), flt(1.25)}}
	v := rec(skill, str("SI_FIREBALL"), num(12), num(0), member(element, 0), list(flag, member(flag, 0), member(flag, 2)),
		boolean(false), &value.Str{V: "both", T: skill.Fields[6].Type}, dur(2000), weights)
	isFree := methods(skill, "isFree", func(r *value.Record) value.Value {
		return boolean(r.Fields[1].(*value.Int).V == 0 && r.Fields[2].(*value.Int).V == 0)
	})
	doc := encode(t, &wire.Document{Schema: "fpdemo.Skill@ae120ca0", Kind: types.List, V: list(skill, v), Methods: isFree})
	want := `{"dwID": "SI_FIREBALL", "legacy": {"reqMp": 12, "reqFp": 0}, "element": 1, "flags": 5, "bTwoHanded": 0, "side": "both", "cast_time": 2, "weights": {"1": 0.5, "2": 1.25}, "$isFree": false}`
	if got := row(t, doc, 0); got != want {
		t.Errorf("row:\n%s\nwant:\n%s", got, want)
	}
	v.Fields[3], v.Fields[5], v.Fields[7] = none(skill.Fields[3].Type), boolean(true), none(skill.Fields[7].Type)
	v.Fields[4] = list(flag)
	want = `{"dwID": "SI_FIREBALL", "legacy": {"reqMp": 12, "reqFp": 0}, "element": 0, "flags": 0, "bTwoHanded": 1, "side": "both", "cast_time": null, "weights": {"1": 0.5, "2": 1.25}, "$isFree": false}`
	doc = encode(t, &wire.Document{Schema: "fpdemo.Skill@ae120ca0", Kind: types.List, V: list(skill, v), Methods: isFree})
	if got := row(t, doc, 0); got != want {
		t.Errorf("none markers, empty bits, int true:\n%s\nwant:\n%s", got, want)
	}
}

// WIRE.md §5.6: `Event.sample` with its inline variant `kind` (tag "type"), compact.
func TestSampleInlineVariant(t *testing.T) {
	weekday := enum("sovcommon.time", "Weekday", "Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat")
	timeOfDay := record("sovcommon.time", "TimeOfDay", field("hour", types.IntType), field("minute", types.IntType))
	window := record("sovcommon.time", "Window", field("day", weekday), field("startUtc", timeOfDay), field("endUtc", timeOfDay))
	item := record("resource.vocab", "Item", field("id", types.StringType, "dwID"))
	items := &types.Collection{Kind: types.CollLet, Pkg: "resource.vocab", Name: "items", Elem: item, KeyedBy: item.Fields[0]}
	itemCount := &types.ListType{Elem: types.IntType}
	kind := &types.VariantType{Pkg: "resource.events", Name: "EventKind", Tag: "type"}
	drop := &types.CaseType{Variant: kind, Name: "monster_drop_inject", Wire: "monster_drop_inject", Fields: record("", "",
		field("itemId", &types.RefType{Target: items}), field("itemCount", itemCount),
		field("levelMin", types.IntType), field("levelMax", types.IntType)).Fields}
	kind.Cases = []*types.CaseType{drop}
	kindField := field("kind", kind)
	kindField.Inline, kindField.WirePath = true, nil
	rollMode := enum("resource.events", "RollMode", "authoritative", "local_budget")
	event := record("resource.events", "Event", field("id", types.StringType), field("worldId", types.UInt32Type),
		field("targetCount", types.IntType), kindField, field("schedule", &types.ListType{Elem: window}), field("rollMode", rollMode))
	at := func(day int, from, to int64) value.Value {
		return rec(window, member(weekday, day), rec(timeOfDay, num(from), num(0)), rec(timeOfDay, num(to), num(0)))
	}
	sample := rec(event, str("sample"), &value.Int{V: 1, T: types.UInt32Type}, num(10),
		rec(drop, ref(items, "II_DEFAULT"), list(types.IntType, num(1), num(1)), num(0), num(0)),
		list(window, at(1, 20, 22)), member(rollMode, 1))
	doc := encode(t, &wire.Document{Schema: "resource.events.Event@00000000", Kind: types.List, V: list(event, sample)})
	want := `{"id": "sample", "worldId": 1, "targetCount": 10, "type": "monster_drop_inject", "itemId": "II_DEFAULT", "itemCount": [1, 1], "levelMin": 0, "levelMax": 0, "schedule": [{"day": "Mon", "startUtc": {"hour": 20, "minute": 0}, "endUtc": {"hour": 22, "minute": 0}}], "rollMode": "local_budget"}`
	if got := row(t, doc, 0); got != want {
		t.Errorf("row:\n%s\nwant:\n%s", got, want)
	}
	kindField.Inline, kindField.WirePath = false, []string{"kind"}
	doc = encode(t, &wire.Document{Schema: "resource.events.Event@00000000", Kind: types.List, V: list(event, sample)})
	want = `{"id": "sample", "worldId": 1, "targetCount": 10, "kind": {"type": "monster_drop_inject", "itemId": "II_DEFAULT", "itemCount": [1, 1], "levelMin": 0, "levelMax": 0}, "schedule": [{"day": "Mon", "startUtc": {"hour": 20, "minute": 0}, "endUtc": {"hour": 22, "minute": 0}}], "rollMode": "local_budget"}`
	if got := row(t, doc, 0); got != want {
		t.Errorf("not inline:\n%s\nwant:\n%s", got, want)
	}
}

// WIRE.md §5.14: element i as k(i) then v(i), slots past the length unwritten.
func TestPairs(t *testing.T) {
	bonus := record("game.items", "StatBonus", field("attribute", types.StringType, "unused"), field("delay", types.DurationType))
	bonus.Fields[1].Unit = types.UnitS
	stats := field("stats", &types.ListType{Elem: bonus})
	stats.Pairs, stats.WirePath = &types.Pairs{Keys: [2]string{"dwDestParam{i}", "nAdjParamVal{i}"}, Slots: 6}, nil
	item := record("game.items", "Item", field("id", types.StringType, "dwID"), stats, field("stack", types.IntType, "dwPackMax"))
	v := rec(item, str("II_X"), list(bonus, rec(bonus, str("DST_STR"), dur(5000)), rec(bonus, str("DST_DEX"), dur(0))), num(1))
	doc := encode(t, &wire.Document{Schema: "game.items.Item@00000000", Kind: types.List, V: list(item, v)})
	want := `{"dwID": "II_X", "dwDestParam0": "DST_STR", "nAdjParamVal0": 5, "dwDestParam1": "DST_DEX", "nAdjParamVal1": 0, "dwPackMax": 1}`
	if got := row(t, doc, 0); got != want {
		t.Errorf("row:\n%s\nwant:\n%s", got, want)
	}
	v.Fields[1] = list(bonus)
	doc = encode(t, &wire.Document{Schema: "game.items.Item@00000000", Kind: types.List, V: list(item, v)})
	if got, want := row(t, doc, 0), `{"dwID": "II_X", "dwPackMax": 1}`; got != want {
		t.Errorf("empty pairs list: %s, want %s", got, want)
	}
}

// WIRE.md §5.5.3 and §5.7: path intermediates in first-use order, a nested retired row.
func TestPathsAndNestedTables(t *testing.T) {
	leaf := record("p", "Leaf", field("x", types.IntType))
	coll := &types.Collection{Kind: types.CollField, Name: "leaves", Elem: leaf}
	r := record("p", "R", field("c", types.IntType, "a", "b", "c"), field("e", types.IntType), field("d", types.IntType, "a", "d"),
		field("t", &types.TableType{Elem: leaf}))
	table := &value.Table{T: r.Fields[3].Type, Entries: []*value.Record{entry(coll, "one", false, rec(leaf, num(1))), entry(coll, "two", true, rec(leaf, num(2)))}}
	got := encode(t, &wire.Document{Schema: "p.R@00000000", Kind: types.Record, V: rec(r, num(1), num(2), num(3), table)})
	want := "{\n  \"$schema\": \"p.R@00000000\",\n  \"value\": {\n    \"a\": {\n      \"b\": {\n        \"c\": 1\n      },\n" +
		"      \"d\": 3\n    },\n    \"e\": 2,\n    \"t\": {\n      \"one\": {\n        \"x\": 1\n      },\n" +
		"      \"two\": {\n        \"$retired\": true,\n        \"x\": 2\n      }\n    }\n  }\n}\n"
	if got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// WIRE.md §5.1, §5.3, §5.4, §5.8, §8.4: values with no wire form are refused, never written.
func TestRefusals(t *testing.T) {
	skill, element, flag := fpdemoSkill()
	cast := field("productionTick", types.DurationType, "productionTickIntervalSec")
	cast.Unit = types.UnitS
	ticker := record("p", "Ticker", cast)
	node := record("p", "TalentNode", field("id", types.IntType), field("parent", &types.OptionalType{Elem: types.IntType}))
	node.Fields[1].NoneWire = []byte("-1")
	key := &types.MapType{Key: &types.LitUnionType{Of: types.StringType, Literals: []string{"default"}}, Value: types.IntType}
	cases := []struct {
		name string
		doc  wire.Document
		want error
	}{
		{"1500ms in s", wire.Document{Kind: types.Record, V: rec(ticker, dur(1500))}, wire.ErrNotWholeUnit},
		{"none marker", wire.Document{Kind: types.Record, V: rec(node, num(7), num(-1))}, wire.ErrNoneMarker},
		{"bits twice", wire.Document{Kind: types.List, V: list(skill, rec(skill, str("S"), num(0), num(0), none(nil),
			list(flag, member(flag, 0), member(flag, 0)), boolean(false), str("both"), none(nil), &value.Map{}))}, wire.ErrBitsRepeated},
		{"key collision", wire.Document{Kind: types.Map, V: &value.Map{T: key, Keys: []value.Value{str("default"), str("default")}, Vals: []value.Value{num(1), num(2)}}}, wire.ErrKeyCollision},
		{"range", wire.Document{Kind: types.Range, V: &value.Range{Start: 1, End: 2, HasEnd: true}}, wire.ErrNoWire},
		{"shape", wire.Document{Kind: types.List, V: member(element, 0)}, wire.ErrShape},
		{"schema", wire.Document{Schema: "Potion@f750790e", Kind: types.Int, V: num(1)}, wire.ErrSchema},
	}
	for _, c := range cases {
		if c.doc.Schema == "" {
			c.doc.Schema = "p.T@00000000"
		}
		if _, err := c.doc.Encode(); !errors.Is(err, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, err, c.want)
		}
	}
}
