package views_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/build"
)

// searched is a public table whose view has a title, a subtitle rendering an enum member by its
// label, and search terms; a local table a ref targets; a local table none does; and French
// texts for the member and the title.
var searched = map[string]string{
	"a/a.canon": `package a

enum Rarity { common, rare }

view Rarity {
  common "Common"
  rare "Rare"
}

record Item {
  name: String
  rarity: Rarity
  alias: String?
}

view Item {
  title "Item {name}"
  subtitle "{rarity}"
  search { name, rarity, alias }
}

let items: table Item = {
  a { name: "Sword", rarity: common }
  b { name: "Sword", rarity: rare, alias: "Blade" }
  retired c { name: "Axe", rarity: rare }
}

local record Tag {
  label: String
}

local let tags: table Tag = {
  t1 { label: "x" }
}

local let lone: table Tag = {
  t2 { label: "y" }
}

record Holder {
  tag: ref tags
  count: Int = 1234567
  note: String? = none
}

view Tag {
  title "Tag {label}"
}

view Holder {
  title "Holds {tag}"
  subtitle "{count:,} {note} {self.note}"
  note { none: "no note" }
}

let holders: table Holder = {
  h1 { tag: t1 }
}

record Via {
  via: ref holders
}

view Via {
  title "Via {via}"
}

let vias: table Via = {
  v1 { via: h1 }
}
`,
	"a/a.fr.canon": `package a
translation fr

Rarity.common "Commun"
Item.title "Objet {name}"
`,
}

// VIEWMODEL.md S1, S2, S8, S9, X4-X6, 12.8: an index per public or ref-targeted table, a row per
// entry (retired flagged) with rendered, disambiguated titles, member labels, refs by title,
// flattened terms, and `tr` where French renders differently.
func TestSearchIndex(t *testing.T) {
	m := tree(t, "  languages: [en, fr]\n", searched, build.Options{}).model(t, demoPkg)
	if _, ok := m.Search["a:lone"]; ok || len(m.Search) != 4 {
		t.Errorf("S1: indexes %v", keysOf(m.Search))
	}
	expect(t, "S1 12.8 tags", m.Search["a:tags"], `{"type":"a.Tag","keyType":"string","count":1,"active":1,"rows":[{"key":"t1","title":"Tag x"}]}`)
	expect(t, "S8 X4 X5", m.Search["a:holders"].Rows, `[{"key":"h1","title":"Holds Tag x","subtitle":"1,234,567 no note no note"}]`)
	expect(t, "S8 one level", m.Search["a:vias"].Rows, `[{"key":"v1","title":"Via Holds t1"}]`)
	expect(t, "S2 S8 S9 X4 X6", m.Search["a:items"], `{"type":"a.Item","keyType":"string","count":3,"active":2,"rows":[
	 {"key":"a","title":"Item Sword (a)","subtitle":"Common","terms":["Sword","common"],
	  "tr":{"fr":{"title":"Objet Sword (a)","subtitle":"Commun"}}},
	 {"key":"b","title":"Item Sword (b)","subtitle":"Rare","terms":["Sword","rare","Blade"],"tr":{"fr":{"title":"Objet Sword (b)"}}},
	 {"key":"c","title":"Item Axe","subtitle":"Rare","terms":["Axe","rare"],"retired":true,"tr":{"fr":{"title":"Objet Axe"}}}]}`)
}

// keysOf are a map's keys, for a failure message.
func keysOf[V any](m map[string]V) []string {
	var out []string
	//canon:unordered a failure message
	for k := range m {
		out = append(out, k)
	}
	return out
}
