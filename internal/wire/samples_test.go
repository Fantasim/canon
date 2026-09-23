package wire_test

import (
	"os"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/wire"
)

// WIRE.md §8.3 (GEN-01): out/potions.json equals examples/pipeline/expected/potions.json.
func TestSamplePotions(t *testing.T) {
	cooldown := field("cooldown", types.DurationType, "dwCooldownMs")
	potion := record("pipeline", "Potion", field("id", types.StringType, "dwID"), field("name", types.StringType, "szName"),
		field("heal", types.IntType, "nHeal"), cooldown, field("stack", types.IntType, "nStack"))
	coll := &types.Collection{Kind: types.CollLet, Pkg: "pipeline", Name: "potions", Elem: potion, KeyedBy: potion.Fields[0]}
	potions := &value.List{T: &types.ListType{Elem: potion, KeyedBy: potion.Fields[0]}, Elems: []value.Value{
		entry(coll, "II_POT_HEAL_L", false, rec(potion, str("II_POT_HEAL_L"), str("IDS_PROPITEM_TXT_POT_L"), num(2500), dur(8000), num(20))),
		entry(coll, "II_POT_HEAL_S", false, rec(potion, str("II_POT_HEAL_S"), str("IDS_PROPITEM_TXT_POT_S"), num(500), dur(3000), num(99))),
	}}
	isStrong := methods(potion, "isStrong", func(r *value.Record) value.Value {
		return boolean(r.Fields[2].(*value.Int).V >= 2000)
	})
	got := encode(t, &wire.Document{Schema: "pipeline.Potion@f750790e", Kind: types.List, V: potions, Methods: isStrong})
	want, err := os.ReadFile("../../examples/pipeline/expected/potions.json")
	if err != nil {
		t.Fatal(err)
	}
	if got != string(want) {
		t.Errorf("potions.json:\n%s\nwant:\n%s", got, want)
	}
	if len(got) != 334 || sha(got) != "2a51028fc9ef1b8b783b4a1c6f8470f61cfc81e06c3f06ef96c738ab7d29bb75" {
		t.Errorf("potions.json: %d bytes, SHA-256 %s", len(got), sha(got))
	}
}

// WIRE.md §8.3: a record value is written `"value": pretty(value, 2)`.
func TestSampleDeck(t *testing.T) {
	layout := &types.Alias{Pkg: "teamboard", Name: "Layout", Def: types.StringType}
	deck := record("teamboard", "Deck", field("layouts", &types.ListType{Elem: layout}), field("maxHidden", types.IntType))
	v := rec(deck, list(layout, str("4:1"), str("2:2")), num(2))
	got := encode(t, &wire.Document{Schema: "teamboard.Deck@02af81fb", Kind: types.Record, V: v})
	want := "{\n  \"$schema\": \"teamboard.Deck@02af81fb\",\n  \"value\": {\n    \"layouts\": [\n      \"4:1\",\n" +
		"      \"2:2\"\n    ],\n    \"maxHidden\": 2\n  }\n}\n"
	if got != want || sha(got) != "3d47a2ebd8ea6ff139c1479016e382b989bc81678df76ee2c974ca2f00c9d87c" {
		t.Errorf("deck.json:\n%s\nwant:\n%s", got, want)
	}
}

// WIRE.md §8.3: a scalar value, an enum of another package.
func TestSampleScalar(t *testing.T) {
	role := enum("sovcommon.roles", "Role", "member", "gm_junior", "gm_senior", "maintainer", "owner", "admin")
	got := encode(t, &wire.Document{Schema: "sovcommon.roles.Role@dc935485", Kind: types.Enum, V: member(role, 3)})
	if want := "{\n  \"$schema\": \"sovcommon.roles.Role@dc935485\",\n  \"value\": \"maintainer\"\n}\n"; got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

// flowStatuses is WIRE.md §8.3's package flow.
func flowStatuses() (*value.Table, wire.Methods, []wire.Fn) {
	status := &types.RecordType{Pkg: "flow", Name: "Status"}
	coll := &types.Collection{Kind: types.CollLet, Pkg: "flow", Name: "statuses", Elem: status}
	status.Fields = record("", "", field("label", types.StringType), field("next", &types.ListType{Elem: &types.RefType{Target: coll}})).Fields
	refs := func(keys ...string) *value.List {
		l := &value.List{T: status.Fields[1].Type}
		for _, k := range keys {
			l.Elems = append(l.Elems, ref(coll, k))
		}
		return l
	}
	table := &value.Table{T: &types.TableType{Elem: status, Stable: true}, Entries: []*value.Record{
		entry(coll, "open", false, rec(status, str("Open"), refs("closed"))),
		entry(coll, "closed", false, rec(status, str("Closed"), refs())),
		entry(coll, "stale", true, rec(status, str("Stale"), refs())),
	}}
	isTerminal := methods(status, "isTerminal", func(r *value.Record) value.Value {
		return boolean(len(r.Fields[1].(*value.List).Elems) == 0)
	})
	domain := []value.Value{ref(coll, "open"), ref(coll, "closed"), ref(coll, "stale")}
	var cells []value.Value
	for _, from := range table.Entries {
		for _, to := range domain {
			contains := false
			for _, n := range from.Fields[1].(*value.List).Elems {
				contains = contains || n.(*value.Ref).Key == to.(*value.Ref).Key
			}
			cells = append(cells, boolean(contains))
		}
	}
	return table, isTerminal, []wire.Fn{{Name: "canTransition", Domains: [][]value.Value{domain, domain}, Cells: cells}}
}

// WIRE.md §8.3: `$id`, `$retired`, a `$` key and `$fns` over a domain with a retired entry.
func TestSampleFlow(t *testing.T) {
	table, isTerminal, fns := flowStatuses()
	got := encode(t, &wire.Document{Schema: "flow.Status@b330a789", Kind: types.Table, V: table, Methods: isTerminal, Fns: fns})
	want := `{
  "$schema": "flow.Status@b330a789",
  "rows": [
    {"$id": "open", "label": "Open", "next": ["closed"], "$isTerminal": false},
    {"$id": "closed", "label": "Closed", "next": [], "$isTerminal": true},
    {"$id": "stale", "$retired": true, "label": "Stale", "next": [], "$isTerminal": true}
  ],
  "$fns": {
    "canTransition": {
      "open": {
        "open": false,
        "closed": true,
        "stale": false
      },
      "closed": {
        "open": false,
        "closed": false,
        "stale": false
      },
      "stale": {
        "open": false,
        "closed": false,
        "stale": false
      }
    }
  }
}
`
	if got != want || sha(got) != "43bc3a5f90c30a159149a5df47092f0e9a4710997841bb53161e64fbc9dd7821" {
		t.Errorf("statuses.json:\n%s\nwant:\n%s", got, want)
	}
	table.Entries = nil
	got = encode(t, &wire.Document{Schema: "flow.Status@b330a789", Kind: types.Table, V: table, Methods: isTerminal})
	if want := "{\n  \"$schema\": \"flow.Status@b330a789\",\n  \"rows\": []\n}\n"; got != want {
		t.Errorf("empty table:\n%s\nwant:\n%s", got, want)
	}
	fns[0].Domains, fns[0].Cells = [][]value.Value{{}, {}}, nil
	got = encode(t, &wire.Document{Schema: "flow.Status@b330a789", Kind: types.Table, V: table, Methods: isTerminal, Fns: fns})
	if want := "  \"rows\": [],\n  \"$fns\": {\n    \"canTransition\": {}\n  }\n}\n"; !strings.HasSuffix(got, want) {
		t.Errorf("empty domains:\n%s", got)
	}
}
