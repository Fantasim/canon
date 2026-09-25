package wire_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/wire"
)

// noneFields has the five columns of WIRE.md §5.4's table, the optional ones marked by -1.
func noneFields(h *host) *types.RecordType {
	a := field("a", opt(types.IntType))
	b := h.withDefault(field("b", opt(types.IntType)), none(opt(types.IntType)))
	c := h.withDefault(field("c", opt(types.IntType)), num(7))
	for _, f := range []*types.Field{a, b, c} {
		f.NoneWire = []byte("-1")
	}
	return record("p", "N", a, b, c, h.withDefault(field("d", types.IntType), num(8)), field("e", types.IntType))
}

var noneCases = []struct {
	json, want string
}{
	{`{"e": 1}`, "N{a: none, b: none, c: 7, d: 8, e: 1}"},
	{`{"a": null, "b": null, "c": null, "e": 1}`, "N{a: none, b: none, c: none, d: 8, e: 1}"},
	{`{"a": -1, "b": -1.0, "c": -1e0, "e": 1}`, "N{a: none, b: none, c: none, d: 8, e: 1}"},
	{`{"a": 5, "b": 6, "c": 2, "d": 3, "e": 4}`, "N{a: 5, b: 6, c: 2, d: 3, e: 4}"},
	{`{"d": null, "e": null}`, findings(at(diag.E3315, "1:7", "/d"), at(diag.E3315, "1:18", "/e"))},
	{`{}`, at(diag.E3302, "1:1", "")},
	{`{"a": "x", "e": 1}`, at(diag.E7110, "1:7", "/a")},
}

// WIRE.md §5.4: absent keys, null and the none marker, per kind of field.
func TestDecodeNone(t *testing.T) {
	h := newHost()
	n := noneFields(h)
	for _, c := range noneCases {
		if got := decodeJSON(t, wire.Decoder{Host: h}, c.json, n).text(); got != c.want {
			t.Errorf("%s: %q, want %q", c.json, got, c.want)
		}
	}
	rewards := field("rewards", opt(&types.MapType{Key: types.StringType, Value: types.IntType}))
	rewards.NoneWire = []byte("{}")
	style := record("resource.adventurequest", "Style", rewards)
	for json, want := range map[string]string{`{"rewards": {}}`: "Style{rewards: none}", `{"rewards": {"a": 1}}`: `Style{rewards: {"a": 1}}`} { //canon:unordered each case alone
		if got := decodeJSON(t, wire.Decoder{}, json, style).text(); got != want {
			t.Errorf("marker {}: %s = %q, want %q", json, got, want)
		}
	}
}

// DECISIONS 173: an optional field with no default asks the host when one is set, else needs none.
func TestDecodeNoneAsksHost(t *testing.T) {
	f := field("a", opt(types.IntType))
	n := record("p", "N", f)
	h := newHost()
	if got := decodeJSON(t, wire.Decoder{Host: h}, `{}`, n).text(); got != "N{a: none}" || h.calls != 1 {
		t.Errorf("with a host: %q after %d calls, want N{a: none} after 1", got, h.calls)
	}
	if got := decodeJSON(t, wire.Decoder{}, `{}`, n).text(); got != "N{a: none}" {
		t.Errorf("without a host: %q, want N{a: none}", got)
	}
}

// WIRE.md §5.5.1, §5.12, §6.4: unknown keys, `$schema` at the root, `partial` at every depth.
func TestDecodeUnknownKeys(t *testing.T) {
	inner := record("p", "In", field("x", types.IntType))
	outer := record("p", "Out", field("in", inner), field("y", types.IntType, "a", "y"))
	cases := []struct {
		partial    bool
		json, want string
	}{
		{false, `{"$schema": "p.Out@00000000", "in": {"x": 1}}`, `Out{in: In{x: 1}, y: "E"}`},
		{false, `{"in": {"x": 1, "z": 2}, "w": 3, "$id": "k"}`, findings(at(diag.E3301, "1:17", "/in/z"), at(diag.E3301, "1:26", "/w"), at(diag.E3301, "1:34", "/$id"))},
		{false, `{"in": {"x": 1, "$schema": 2}}`, at(diag.E3301, "1:17", "/in/$schema")},
		{false, `{"in": {"x": 1}, "a": {"y": 2, "q": 3}}`, at(diag.E3301, "1:32", "/a/q")},
		{true, `{"in": {"x": 1, "z": 2}, "a": {"y": 2, "q": 3}, "w": 1}`, "Out{in: In{x: 1}, y: 2}"},
	}
	outer.Fields[1].Default = &syntax.IntLit{}
	h := newHost()
	h.defaults[outer.Fields[1]] = str("E")
	for _, c := range cases {
		got := decodeJSON(t, wire.Decoder{Host: h, Partial: c.partial}, c.json, outer).text()
		if got != c.want {
			t.Errorf("%s: %q, want %q", c.json, got, c.want)
		}
	}
}

// WIRE.md §5.5.3: key paths, a missing intermediate absent, a non-object one E7110.
func TestDecodePaths(t *testing.T) {
	h := newHost()
	skill := record("fpdemo", "Skill", field("id", types.StringType, "dwID"),
		h.withDefault(field("reqMp", types.IntType, "legacy", "reqMp"), num(0)),
		h.withDefault(field("reqFp", types.IntType, "legacy", "reqFp"), num(0)))
	cases := []struct{ json, want string }{
		{`{"dwID": "S", "legacy": {"reqMp": 12}}`, `Skill{id: "S", reqMp: 12, reqFp: 0}`},
		{`{"dwID": "S"}`, `Skill{id: "S", reqMp: 0, reqFp: 0}`},
		{`{"dwID": "S", "legacy": null}`, at(diag.E7110, "1:25", "/legacy")},
		{`{"dwID": 1, "legacy": {"reqMp": "x", "reqFp": true}}`, findings(at(diag.E7110, "1:10", "/dwID"), at(diag.E7110, "1:33", "/legacy/reqMp"), at(diag.E7110, "1:47", "/legacy/reqFp"))},
	}
	for _, c := range cases {
		if got := decodeJSON(t, wire.Decoder{Host: h}, c.json, skill).text(); got != c.want {
			t.Errorf("%s: %q, want %q", c.json, got, c.want)
		}
	}
}

// WIRE.md §5.5.4, TYP-17: an input field has no wire form; its key in the data is E3312.
func TestDecodeInput(t *testing.T) {
	key := field("apiKey", opt(types.StringType))
	key.Input = &types.Input{Env: "KEY"}
	r := record("p", "Gen", field("x", types.IntType), key)
	if got := decodeJSON(t, wire.Decoder{}, `{"x": 1}`, r); !got.ok || got.v.(*value.Record).Fields[1] != nil {
		t.Errorf("input field: %q", got.text())
	}
	if got := decodeJSON(t, wire.Decoder{Partial: true}, `{"x": 1, "apiKey": "k"}`, r).text(); got != at(diag.E3312, "1:10", "/apiKey") {
		t.Errorf("input key: %q", got)
	}
}

// WIRE.md §5.6: a variant is an object whose tag names the case; inline, the parent holds both.
func TestDecodeVariants(t *testing.T) {
	h := newHost()
	event := eventFixture(h)
	kind := event.Fields[3].Type
	cases := []struct {
		typ        types.Type
		json, want string
	}{
		{kind, `{"type": "monster_drop_inject", "itemId": "II_X"}`, "monster_drop_inject{itemId: II_X, itemCount: [1, 1], levelMin: 0, levelMax: 0}"},
		{kind, `{"itemId": "II_X"}`, at(diag.E7112, "1:1", "")},
		{kind, `{"type": 3}`, at(diag.E7110, "1:10", "/type")},
		{kind, `{"type": "spawn_item"}`, at(diag.E7112, "1:10", "/type")},
		{kind, `{"type": "monster_drop_inject", "itemId": "I", "monsterId": "M"}`, at(diag.E3301, "1:48", "/monsterId")},
		{event, `{"id": "e", "worldId": 1, "targetCount": 2, "type": "spawn_monster", "monsterId": "MI_X", "spawnRegion": {"left": 1, "top": 2, "right": 3, "bottom": 4}, "schedule": []}`,
			`Event{id: "e", worldId: 1, targetCount: 2, kind: spawn_monster{monsterId: MI_X, spawnRegion: Rect{left: 1, top: 2, right: 3, bottom: 4}, monsterLifetime: none}, schedule: [], rollMode: local_budget}`},
		{event, `{"id": "e", "worldId": 1, "targetCount": 2, "schedule": [], "itemId": "I", "levelMin": 3, "$isFree": true}`, at(diag.E7112, "1:1", "")},
		{event, `{"id": "e", "worldId": 1, "targetCount": 2, "schedule": [], "type": 7, "monsterId": "M"}`, at(diag.E7110, "1:69", "/type")},
		{event, `{"id": "e", "worldId": 1, "targetCount": 2, "type": "spawn_monster", "itemId": "I", "schedule": []}`,
			findings(at(diag.E3302, "1:1", ""), at(diag.E3302, "1:1", ""), at(diag.E3301, "1:70", "/itemId"))},
	}
	for _, c := range cases {
		if got := decodeJSON(t, wire.Decoder{Host: h}, c.json, c.typ).text(); got != c.want {
			t.Errorf("%s: %q, want %q", c.json, got, c.want)
		}
	}
}

// WIRE.md §5.14: filled slots, contiguous from 0; one key, a null or a gap is E7117.
func TestDecodePairs(t *testing.T) {
	bonus := record("game.items", "StatBonus", field("attribute", types.StringType, "unused"), field("delay", types.DurationType))
	bonus.Fields[1].Unit = types.UnitS
	stats := field("stats", listOf(bonus))
	stats.Pairs, stats.WirePath = &types.Pairs{Keys: [2]string{"k{i}", "v{i}"}, Slots: 3}, nil
	item := record("game.items", "Item", stats)
	cases := []struct{ json, want string }{
		{`{"k0": "STR", "v0": 5, "k1": "DEX", "v1": 0.5}`, `Item{stats: [StatBonus{attribute: "STR", delay: 5s}, StatBonus{attribute: "DEX", delay: 500ms}]}`},
		{`{}`, "Item{stats: []}"},
		{`{"k0": "STR"}`, at(diag.E7117, "1:2", "/k0")},
		{`{"v1": 1, "k0": "A", "v0": 1}`, at(diag.E7117, "1:2", "/v1")},
		{`{"k0": null, "v0": 1}`, at(diag.E7117, "1:2", "/k0")},
		{`{"k1": "A", "v1": 1, "k2": "B", "v2": 2}`, findings(at(diag.E7117, "1:2", "/k1"), at(diag.E7117, "1:22", "/k2"))},
		{`{"k01": "A", "v01": 1}`, findings(at(diag.E3301, "1:2", "/k01"), at(diag.E3301, "1:14", "/v01"))},
		{`{"k3": "A", "v3": 1}`, findings(at(diag.E3301, "1:2", "/k3"), at(diag.E3301, "1:13", "/v3"))},
		{`{"k0": 1, "v0": "x"}`, findings(at(diag.E7110, "1:8", "/k0"), at(diag.E7110, "1:17", "/v0"))},
	}
	for _, c := range cases {
		if got := decodeJSON(t, wire.Decoder{}, c.json, item).text(); got != c.want {
			t.Errorf("%s: %q, want %q", c.json, got, c.want)
		}
	}
	stats.Pairs.Slots = 1_000_000_000
	if got := decodeJSON(t, wire.Decoder{}, `{"k999999999": "A", "v999999999": 1}`, item).text(); got != at(diag.E7117, "1:2", "/k999999999") {
		t.Errorf("a huge bound reads the members once: %q", got)
	}
}

// EVALUATION.md §7.1: a failed record skips its later defaults; the next element does not.
func TestDecodeNoDefaultAfterFailure(t *testing.T) {
	h := newHost()
	r := record("p", "R", field("a", types.IntType), h.withDefault(field("b", types.IntType), num(1)), field("c", types.IntType))
	got := decodeJSON(t, wire.Decoder{Host: h}, `[{"a": "x", "c": 1}, {"a": 1, "c": "y"}]`, listOf(r)).text()
	if got != findings(at(diag.E7110, "1:8", "/0/a"), at(diag.E7110, "1:36", "/1/c")) || h.calls != 1 {
		t.Errorf("%q after %d default calls, want 1", got, h.calls)
	}
}
