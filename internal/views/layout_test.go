package views_test

import (
	"reflect"
	"testing"
)

// layouts is a record with named groups, `show` lines, methods, an inline variant whose case
// fields are placed by name, a deprecated field named in a group, and text properties.
const layouts = `package a

enum Rarity { common, rare }

variant Kind {
  weapon { attack: Int, speed: Int }
  food { heal: Int, speed: Int }
}

record Stats {
  hp: Int
}

record Item {
  /// What it is called.
  name: String
  kind: Kind @json(inline)
  rarity: Rarity = common
  weight: Int? = 1
  stats: Stats
  steps: [Int]
  secret: Int = 0
  shown: Bool = false
  fixed: Int(3..=3)
  old: Int = 0 @deprecated("Gone")

  fn doubled(self) -> Int {
    return 2
  }

  fn tripled(self) -> Int {
    return 3
  }
}

view Item {
  title "Item {name}"
  singular "an item"
  show "Summary" "{name}"
  tripled "Thrice"
  doubled "Twice"
  name "Name" { placeholder: "Type a name", help: "The name." }
  weight { none: "No weight", when: shown }
  steps { step: "Step {index}" }
  secret { hidden: true }
  group main "Main" "The basics." { name, rarity, speed, old }
  group body "Body" advanced when shown { stats }
  group extra "Extra" { show calc "Calc" "{name}", doubled { hidden: true } }
}

view Kind.weapon {
  title "Weapon"
}
`

// expect compares v, as JSON, with want; rule names what it proves.
func expect(t *testing.T, rule string, v any, want string) {
	t.Helper()
	if got := canonical(t, v); !reflect.DeepEqual(got, decode(t, []byte(want))) {
		t.Errorf("%s:\n got %s\nwant %s", rule, text(v), want)
	}
}

// VIEWMODEL.md L1, L3-L6, L16-L18, L22, 12.4: named groups in view order (case fields expanded,
// deprecated ones left out, a lone section flattened), `_other` with the view-level lines no
// group names (L22) and the fields no group places, then More and Unused fields.
func TestRecordSections(t *testing.T) {
	v := demo(t, layouts, "").model(t, demoPkg).Views["a.Item"]
	expect(t, "L1 L3 L4 L5 L6 L16 L18 L22", v.Sections, `[
	 {"kind":"group","id":"main","label":"a:Item.group.main","intro":"a:Item.group.main.intro",
	  "entries":[{"field":"name"},{"field":"rarity"},{"field":"kind.weapon.speed"},{"field":"kind.food.speed"}]},
	 {"kind":"group","id":"body","label":"a:Item.group.body","advanced":true,"when":"shown","flatten":true,
	  "entries":[{"field":"stats"}]},
	 {"kind":"group","id":"extra","label":"a:Item.group.extra","entries":[{"show":"calc"},{"method":"doubled"}]},
	 {"kind":"other","entries":[{"show":"_0"},{"method":"tripled"}],
	  "fields":["kind","kind.weapon.attack","kind.food.heal","weight","steps","secret","shown","fixed"]},
	 {"kind":"more"},
	 {"kind":"unused","fields":["old"]}]`)
	if !v.Declared || v.Kind != "record" {
		t.Errorf("12.4: kind %q, declared %v", v.Kind, v.Declared)
	}
	expect(t, "12.4 title, singular (J9)", []any{v.Title, v.Singular}, `[{"template":"Item {name}","text":"a:Item.title"},"a:Item.singular"]`)
}

// VIEWMODEL.md 12.4 field view, X1, L17, L18, C34, C43, 3.5: each field view: its label (the
// view's, else humanized) and help, its texts, `hidden` and `when`, a case field's case path and
// relative path and its case's keys (I18N.md K7), a single value.
func TestFieldViews(t *testing.T) {
	f := demo(t, layouts, "").model(t, demoPkg).Views["a.Item"].Fields
	cases := []struct{ key, want, rule string }{
		{"name", `{"label":"a:Item.name","help":"a:Item.name.help","control":{"kind":"input"},"placeholder":"a:Item.name.placeholder"}`, "X1 3.5 placeholder"},
		{"weight", `{"label":"a:Item.weight","control":{"kind":"number","optional":{"unset":"clear","threeState":true}},"when":"shown","none":"a:Item.weight.none"}`, "X1 C36 L14 C38"},
		{"steps", `{"label":"a:Item.steps","control":{"kind":"tags","element":{"kind":"number"},"orderable":true},"step":"a:Item.steps.step"}`, "T24"},
		{"secret", `{"label":"a:Item.secret","control":{"kind":"number"},"hidden":true}`, "L13"},
		{"kind.weapon.attack", `{"label":"a:Kind.weapon.attack","control":{"kind":"number"},"case":"kind=weapon","path":"kind.attack"}`, "L17 L18 K7"},
		{"fixed", `{"label":"a:Item.fixed","control":{"kind":"number","min":3,"max":3,"stepper":true},"readonly":"single","single":3}`, "C34 C43"},
		{"old", `{"label":"a:Item.old","control":{"kind":"number"},"readonly":"deprecated"}`, "L12 C43"},
	}
	for _, c := range cases {
		expect(t, c.rule+" "+c.key, f[c.key], c.want)
	}
}

// VIEWMODEL.md L21, L23, 12.4 methods and shows: the methods the view names, labelled by the
// view or humanized, `hidden`; the `show` lines by id, unnamed ones `_0`, … (G17).
func TestMethodsAndShows(t *testing.T) {
	v := demo(t, layouts, "").model(t, demoPkg).Views["a.Item"]
	expect(t, "L23", v.Methods, `{"doubled":{"label":"a:Item.doubled","hidden":true},"tripled":{"label":"a:Item.tripled"}}`)
	expect(t, "L21 G17", v.Shows, `{"_0":{"label":"a:Item.show._0","text":{"template":"{name}"}},
	 "calc":{"label":"a:Item.show.calc","text":{"template":"{name}"}}}`)
}

// VIEWMODEL.md 12.4 variant view, C9, D5: a variant's view holds its case selector and a view per
// case, laid out by `view V.c` when there is one, else by the default layout.
func TestVariantView(t *testing.T) {
	v := demo(t, layouts, "").model(t, demoPkg).Views["a.Kind"]
	expect(t, "C9", v.Selector, `{"kind":"segmented","source":{"cases":"a.Kind"}}`)
	w, f := v.Cases["weapon"], v.Cases["food"]
	if v.Declared || !w.Declared || f.Declared || w.Kind != "case" {
		t.Errorf("D5: variant declared %v, weapon %v %q, food %v", v.Declared, w.Declared, w.Kind, f.Declared)
	}
	expect(t, "D5 weapon", []any{w.Title, w.Sections[0]}, `[{"template":"Weapon","text":"a:Kind.weapon.title"},{"kind":"other","fields":["attack","speed"]}]`)
	expect(t, "D5 food", f.Fields["heal"], `{"label":"a:Kind.food.heal","control":{"kind":"number"}}`)
}
