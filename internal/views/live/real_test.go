package live_test

import (
	"strconv"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/views/live"
	"github.com/fantasim/canonlang/internal/views/render"
)

// measured is a table whose view renders enum labels and a ref's title in a `show` line and in
// view-named methods, and loops past the project budget in one line and one method; French
// translates one label, the title and the line's template.
var measured = map[string]string{
	"a/a.canon": `package a

enum Goal { kill, collect }

view Goal {
  kill "Kill"
  collect "Collect"
}

record Ore {
  name: String
}

view Ore {
  title "Ore {name}"
}

record Thing {
  name: String
  goal: Goal
  ore: ref ores
  note: String?

  fn aim(self) -> Goal { return goal }

  fn source(self) -> ref ores { return ore }

  fn memo(self) -> String? { return note }

  fn spin(self) -> Int {
    var i = 0
    while i < 100000 { i += 1 }
    return i
  }
}

view Thing {
  title "Thing {name}"
  show "Sum" "{name} is {goal} from {ore}"
  show spun "Spun" "{spin()}"
  aim "Aim"
  source "Source"
  memo "Memo"
  spin "Spin"
}

let ores: table Ore = {
  o1 { name: "Rock" }
}

let things: table Thing = {
  t1 { name: "A", goal: kill, ore: o1 }
  t2 { name: "B", goal: collect, ore: o1 }
}
`,
	"a/a.fr.canon": `package a
translation fr

Goal.kill "Tuer"
Thing.title "Chose {name}"
Thing.show._0.text "{name} : {goal}, {ore}"
`,
}

// measuredProject is measured, analyzed with a budget a line and a method exceed.
func measuredProject(t *testing.T) *analyzed {
	t.Helper()
	return demo(t, "  languages: [en, fr]\n  budget: 20000\n", measured)
}

// texts are the Text of each show line of res.
func texts(res *live.Result) []live.Text {
	out := make([]live.Text, 0, len(res.Show))
	for _, l := range res.Show {
		out = append(out, l.Text)
	}
	return out
}

// API.md V5, V10, V11 (VIEWMODEL.md L21-L23, X4): with the analysis's evaluator, every `show` line
// and view-named method of the form renders: a template's interpolations, a method's value.
func TestRealShow(t *testing.T) {
	p := shopProject(t)
	res := at(t, p.real(), live.Target{Value: p.item(t, "sword"), Name: "sword"})
	same(t, "API.md V10 API.md V11", texts(res), `[
	 {"Value":"Sword x3","OK":true,"Fallback":false}, {"Value":"3","OK":true,"Fallback":false},
	 {"Value":"6","OK":true,"Fallback":false}, {"Value":"3","OK":true,"Fallback":false},
	 {"Value":"Sword","OK":true,"Fallback":false}, {"Value":"3","OK":true,"Fallback":false},
	 {"Value":"sword","OK":true,"Fallback":false}, {"Value":"2","OK":true,"Fallback":false}]`)
	same(t, "API.md V7", []any{res.Title, res.Subtitle}, `[{"Value":"Item Sword","OK":true,"Fallback":false},{"Value":"kill","OK":true,"Fallback":false}]`)
	step := fieldValue(t, p.item(t, "sword"), "steps").(*value.List).Elems[0]
	placed := at(t, p.real(), live.Target{Value: step, Magic: render.Magic{Index: intValue(2)}})
	same(t, "API.md V10 VIEWMODEL.md 3.4", placed.Show[0].Text, `{"Value":"2","OK":true,"Fallback":false}`)
	if err := p.a.ViewErr(); err != nil {
		t.Errorf("API.md X2: %v", err)
	}
}

// API.md V8, V10 (VIEWMODEL.md X4, X6, S8; log-2026-09-29 M4 U9): in French a `show` line renders
// its translated template, falling back through a ref's title; a method's enum label or ref's
// title falls back when untranslated; a method's `none` renders `none`, no fallback.
func TestRealFallback(t *testing.T) {
	p := measuredProject(t)
	t1 := at(t, p.real(), live.Target{Value: entry(t, p.let(t, "a", "things"), "t1"), Name: "t1", Lang: "fr"})
	t2 := at(t, p.real(), live.Target{Value: entry(t, p.let(t, "a", "things"), "t2"), Name: "t2", Lang: "fr"})
	same(t, "API.md V8 API.md V10", []any{t1.Title, t1.Show[0], t1.Show[2].Text, t2.Show[2].Text, t1.Show[3].Text, t1.Show[4].Text}, `[
	 {"Value":"Chose A","OK":true,"Fallback":false},
	 {"Owner":"","Key":"Thing.show._0","Label":"Sum","Text":{"Value":"A : Tuer, Ore Rock","OK":true,"Fallback":true}},
	 {"Value":"Tuer","OK":true,"Fallback":false}, {"Value":"Collect","OK":true,"Fallback":true},
	 {"Value":"Ore Rock","OK":true,"Fallback":true}, {"Value":"none","OK":true,"Fallback":false}]`)
	source := at(t, p.real(), live.Target{Value: entry(t, p.let(t, "a", "things"), "t1"), Name: "t1"})
	same(t, "API.md V8 source", []any{source.Show[0].Text, source.Show[2].Text}, `[
	 {"Value":"A is Kill from Ore Rock","OK":true,"Fallback":false}, {"Value":"Kill","OK":true,"Fallback":false}]`)
}

// API.md V11, V13 (EVALUATION.md 12.2, DECISIONS 195): a `show` line or method whose evaluation
// runs past the budget's size is not OK, and the lines after it still render.
func TestRealBudget(t *testing.T) {
	p := measuredProject(t)
	res := at(t, p.real(), live.Target{Value: entry(t, p.let(t, "a", "things"), "t1"), Name: "t1"})
	var keys []string
	for _, l := range res.Show {
		keys = append(keys, l.Key)
	}
	same(t, "API.md V10", keys, `["Thing.show._0","Thing.show.spun","Thing.aim","Thing.source","Thing.memo","Thing.spin"]`)
	same(t, "API.md V11 API.md V13", []any{res.Show[1].Text, res.Show[5].Text, res.Show[4].Text}, `[
	 {"Value":"","OK":false,"Fallback":false}, {"Value":"","OK":false,"Fallback":false},
	 {"Value":"none","OK":true,"Fallback":false}]`)
	if err := p.a.ViewErr(); err != nil {
		t.Errorf("API.md X2: %v", err)
	}
	roomy := demo(t, "  languages: [en, fr]\n", measured)
	within := at(t, roomy.real(), live.Target{Value: entry(t, roomy.let(t, "a", "things"), "t1"), Name: "t1"})
	same(t, "API.md V13 within the default budget", []any{within.Show[1].Text, within.Show[5].Text}, `[
	 {"Value":"100000","OK":true,"Fallback":false}, {"Value":"100000","OK":true,"Fallback":false}]`)
}

// TYPES.md 11.1, API.md 11 `Types` (log-2026-09-29 M4 U9): at a nested record or an element
// whose arguments its enclosing record binds, above the path, the analysis's bound arguments
// resolve its dependent fields; without them the entry is left out.
func TestRealBound(t *testing.T) {
	p := shopProject(t)
	sw := p.item(t, "sword")
	plan := fieldValue(t, sw, "plan")
	element := fieldValue(t, sw, "plans").(*value.List).Elems[0]
	unbound := p.real()
	unbound.Bound = nil
	num := `{"aim":{"kind":"int","bits":64,"signed":true}}`
	got := []any{at(t, p.real(), live.Target{Value: element}).Types, at(t, p.real(), live.Target{Value: plan}).Types, at(t, unbound, live.Target{Value: plan}).Types}
	same(t, "API.md 11 Types bound", got, `[`+num+`,`+num+`,{}]`)
}

// API.md V5, V7, V10 over examples/game/items, in French: a weapon's title calls the method
// `name` (`{name()}`), which its view's group names too (a method line); its kind's case view
// adds a `show` line owned by `kind`, labelled in French, its text the average of its attacks.
func TestExampleItemsShow(t *testing.T) {
	p := examples(t)
	weapon := firstWeapon(t, p.let(t, "game.items", "items").(*value.Table))
	kind := fieldValue(t, weapon, "kind")
	lo, _ := strconv.Atoi(fieldValue(t, kind, "attackMin").CanonText())
	hi, _ := strconv.Atoi(fieldValue(t, kind, "attackMax").CanonText())
	res := at(t, p.real(), live.Target{Value: weapon, Name: weapon.Ident.Key.Text(), Lang: "fr"})
	if !res.Title.OK || res.Title.Value == "" || strings.Contains(res.Title.Value, "{") {
		t.Errorf("API.md V7: title %v", res.Title)
	}
	same(t, "API.md V10", res.Show, `[{"Owner":"","Key":"Item.name","Label":"Name",
	 "Text":{"Value":"`+res.Title.Value+`","OK":true,"Fallback":false}}, {"Owner":"kind","Key":"ItemKind.IK1_WEAPON.show._0","Label":"Coup moyen",
	 "Text":{"Value":"`+strconv.Itoa((lo+hi)/2)+`","OK":true,"Fallback":false}}]`)
	if err := p.a.ViewErr(); err != nil {
		t.Errorf("API.md X2: %v", err)
	}
	if li := p.a.LiveInputs(); li.Studio != studioPkg || li.I18N["game.items"] == nil {
		t.Errorf("API.md 11: live inputs %q, %v", li.Studio, li.I18N["game.items"])
	}
}

// firstWeapon is the first entry of items whose kind is the case IK1_WEAPON.
func firstWeapon(t *testing.T, items *value.Table) *value.Record {
	t.Helper()
	for _, e := range items.Entries {
		k, ok := fieldValue(t, e, "kind").(*value.Record)
		if c, isCase := k.T.Base().(*types.CaseType); ok && isCase && c.Name == "IK1_WEAPON" {
			return e
		}
	}
	t.Fatal("no IK1_WEAPON item")
	return nil
}
