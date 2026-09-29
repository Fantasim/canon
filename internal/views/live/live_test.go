package live_test

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/views/live"
	"github.com/fantasim/canonlang/internal/views/render"
)

// at is the live state of the target, read with in.
func at(t *testing.T, in live.Input, target live.Target) *live.Result {
	t.Helper()
	res, err := live.Evaluate(context.Background(), in, target)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

// evaluate is the live state of the value v named name, in lang.
func evaluate(t *testing.T, p *analyzed, v value.Value, name, lang string) *live.Result {
	t.Helper()
	return at(t, p.input(), live.Target{Value: v, Name: name, Lang: lang})
}

// shopProject is shop, analyzed.
func shopProject(t *testing.T) *analyzed {
	t.Helper()
	return demo(t, languages, shop)
}

// sword is the live state of the entry sword of `items`, in lang.
func sword(t *testing.T, lang string) (*analyzed, *live.Result) {
	t.Helper()
	p := shopProject(t)
	return p, evaluate(t, p, p.item(t, "sword"), "sword", lang)
}

// item is the entry key of `items`.
func (p *analyzed) item(t *testing.T, key string) *value.Record {
	t.Helper()
	return entry(t, p.let(t, "a", "items"), key)
}

// same fails unless got, as JSON, equals want.
func same(t *testing.T, rule string, got any, want string) {
	t.Helper()
	var g, w any
	if err := json.Unmarshal([]byte(jsonOf(t, got)), &g); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(want), &w); err != nil {
		t.Fatalf("%s: bad want: %v", rule, err)
	}
	if !reflect.DeepEqual(g, w) {
		t.Errorf("%s:\n got %s\nwant %s", rule, jsonOf(t, got), want)
	}
}

// fieldValue is the field name of the record v.
func fieldValue(t *testing.T, v value.Value, name string) value.Value {
	t.Helper()
	f, ok := fieldOf(v, name)
	if !ok {
		t.Fatalf("%s has no field %s", v.Type(), name)
	}
	return f
}

// API.md V5, V9 (VIEWMODEL.md Q2, L14, L18): every `when` of the form, keyed by relative path:
// fields and methods of the value and of the records and cases its fields hold, an inline case
// field by its value path, groups by `<record path>#<id>`; false only when it evaluates false.
func TestWhen(t *testing.T) {
	_, res := sword(t, "")
	same(t, "API.md V5 API.md V9", res.When, `{
	 "doubled": false, "count": false, "kind.speed": true, "#main": false, "#rest": true,
	 "stats.hp": true, "stats#vitals": true, "kind.grip.hp": true, "kind.grip#vitals": true,
	 "effect.turns": false}`)
}

// API.md V12 (VIEWMODEL.md G12): a condition reading a magic name the position gives no value
// fails, and counts as true; with the value it holds or not.
func TestWhenFails(t *testing.T) {
	p := shopProject(t)
	step := fieldValue(t, p.item(t, "sword"), "steps").(*value.List).Elems[0]
	var got []bool
	for _, m := range []render.Magic{{}, {Index: intValue(1)}, {Index: intValue(2)}} {
		got = append(got, at(t, p.input(), live.Target{Value: step, Magic: m}).When["text"])
	}
	same(t, "API.md V12", got, `[true, false, true]`)
}

// API.md V10, V11 (VIEWMODEL.md Q1, L21-L23, G17): one line per `show` item and view-named
// method of every value of the form, in view order, nested views after their owner, keyed by
// their label key; a method named at view level and in a group only in the group (L22).
func TestShow(t *testing.T) {
	_, res := sword(t, "")
	same(t, "API.md V10 API.md V11", res.Show, `[
	 {"Owner":"","Key":"Item.show._0","Label":"Summary","Text":{"Value":"Sword x3","OK":true,"Fallback":false}},
	 {"Owner":"","Key":"Item.show.power","Label":"Power","Text":{"Value":"3","OK":true,"Fallback":false}},
	 {"Owner":"","Key":"Item.doubled","Label":"Twice","Text":{"Value":"doubled()","OK":true,"Fallback":false}},
	 {"Owner":"","Key":"Item.broken","Label":"Broken","Text":{"Value":"","OK":false,"Fallback":false}},
	 {"Owner":"","Key":"Item.show._1","Label":"Extra","Text":{"Value":"Sword","OK":true,"Fallback":false}},
	 {"Owner":"","Key":"Item.show._2","Label":"Tail","Text":{"Value":"3","OK":true,"Fallback":false}},
	 {"Owner":"","Key":"Item.show.ident","Label":"Ident","Text":{"Value":"sword","OK":true,"Fallback":false}},
	 {"Owner":"effect","Key":"Effect.freeze.show._0","Label":"Turns","Text":{"Value":"2","OK":true,"Fallback":false}}]`)
}

// API.md V11 (VIEWMODEL.md G12): a `show` line reading a magic name its position gives no value
// is not OK and empty.
func TestShowFails(t *testing.T) {
	p := shopProject(t)
	step := fieldValue(t, p.item(t, "sword"), "steps").(*value.List).Elems[0]
	bare := at(t, p.input(), live.Target{Value: step})
	placed := at(t, p.input(), live.Target{Value: step, Magic: render.Magic{Index: intValue(1)}})
	same(t, "API.md V11", []any{bare.Show[0].Text, placed.Show[0].Text}, `[
	 {"Value":"","OK":false,"Fallback":false},{"Value":"1","OK":true,"Fallback":false}]`)
}

// API.md V8 (I18N.md B1; log-2026-09-29 M4 U9): in French a translated title, label or template
// is used; an untranslated text falls back with Fallback set, and so does a text interpolating
// an enum label or a ref's target title that falls back; a text without a key does not.
func TestFallback(t *testing.T) {
	p, res := sword(t, "fr")
	same(t, "API.md V8 title", res.Title, `{"Value":"Objet Sword","OK":true,"Fallback":false}`)
	same(t, "API.md V8 nested label", res.Subtitle, `{"Value":"kill","OK":true,"Fallback":true}`)
	summary, power, ident := res.Show[0], res.Show[1], res.Show[6]
	same(t, "API.md V8 show", []any{summary.Text, power.Label, power.Text, ident.Text}, `[
	 {"Value":"Sword x3","OK":true,"Fallback":true}, "Puissance",
	 {"Value":"3","OK":true,"Fallback":false}, {"Value":"sword","OK":true,"Fallback":false}]`)
	same(t, "API.md V8 cells", res.Headings["parts[0]"].Cells["stats"], `{"Value":"Statistiques 1","OK":true,"Fallback":false}`)
	same(t, "API.md V8 nested ref", res.Headings["parts[0]"].Subtitle, `{"Value":"Metal Iron of o1","OK":true,"Fallback":true}`)
	metals := evaluate(t, p, p.let(t, "a", "metals"), "metals", "fr")
	same(t, "API.md V8 own", metals.Headings["iron"].Title, `{"Value":"Metal Iron of Minerai","OK":true,"Fallback":true}`)
}

// API.md V8 (I18N.md B3; log-2026-09-29 M4 U9): a language without a translation file falls
// back for every text that has a key.
func TestFallbackNoFile(t *testing.T) {
	_, res := sword(t, "de")
	same(t, "API.md V8 no file", []any{res.Title, res.Show[1].Label, res.Show[0].Text}, `[
	 {"Value":"Item Sword","OK":true,"Fallback":true}, "Power", {"Value":"Sword x3","OK":true,"Fallback":true}]`)
}

// API.md V7, V11 (VIEWMODEL.md S8; log-2026-09-29 M4 U9): a title from the view, a ref in it by
// its target's title (refs in that one by key), `{x.id}` by key; a plain element without a view
// title `#<n>`; a value's name; an absent subtitle empty but OK; a failing title not OK.
func TestTitles(t *testing.T) {
	p, res := sword(t, "")
	same(t, "API.md V7 view", []any{res.Title, res.Subtitle, res.Preview}, `[
	 {"Value":"Item Sword","OK":true,"Fallback":false},{"Value":"kill","OK":true,"Fallback":false},""]`)
	same(t, "API.md V7 ref", res.Headings["parts[0]"].Subtitle, `{"Value":"Metal Iron of o1","OK":true,"Fallback":false}`)
	same(t, "API.md V7 literal", res.Headings["parts[1]"].Subtitle, `{"Value":"none","OK":true,"Fallback":false}`)
	metals := evaluate(t, p, p.let(t, "a", "metals"), "metals", "")
	same(t, "API.md V7 id", metals.Headings["gold"].Subtitle, `{"Value":"gold / o1","OK":true,"Fallback":false}`)
	same(t, "API.md V7 no view", res.Headings["notes[0]"], `{"Title":{"Value":"#1","OK":true,"Fallback":false},
	 "Subtitle":{"Value":"","OK":true,"Fallback":false},"Preview":"","Retired":false,"Cells":{}}`)
	notes := evaluate(t, p, fieldValue(t, p.item(t, "sword"), "notes"), "notes", "")
	same(t, "API.md V7 value name", []any{notes.Title, notes.Subtitle}, `[
	 {"Value":"notes","OK":true,"Fallback":false},{"Value":"","OK":true,"Fallback":false}]`)
	stats := evaluate(t, p, fieldValue(t, p.item(t, "sword"), "stats"), "stats", "")
	same(t, "API.md V7 no subtitle", stats.Subtitle, `{"Value":"","OK":true,"Fallback":false}`)
	slot := fieldValue(t, p.item(t, "sword"), "slots").(*value.Map).Vals[0]
	same(t, "API.md V11 title", evaluate(t, p, slot, "belt", "").Title, `{"Value":"","OK":false,"Fallback":false}`)
}

// VIEWMODEL.md 3.4, G12: the magic names of the value at the path are the caller's: a map
// value's `key` renders; without it the title fails.
func TestMagic(t *testing.T) {
	p := shopProject(t)
	slots := fieldValue(t, p.item(t, "sword"), "slots").(*value.Map)
	got := at(t, p.input(), live.Target{Value: slots.Vals[0], Magic: render.Magic{Key: slots.Keys[0]}, Name: "belt"})
	same(t, "3.4 G12", got.Title, `{"Value":"Slot belt","OK":true,"Fallback":false}`)
}

// A cancelled context stops Evaluate with its error.
func TestCancelled(t *testing.T) {
	p := shopProject(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := live.Evaluate(ctx, p.input(), live.Target{Value: p.let(t, "a", "items")})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

// API.md V11, V12: without an Evaluator and Methods, every rendering fails, every condition holds,
// and nothing panics.
func TestNoEvaluator(t *testing.T) {
	p := shopProject(t)
	in := p.input()
	in.Eval, in.Methods = nil, nil
	res := at(t, in, live.Target{Value: p.item(t, "sword")})
	if res.Title.OK || res.Show[0].Text.OK || !res.When["count"] {
		t.Errorf("API.md V11 API.md V12: title %v, show %v, when %v", res.Title, res.Show[0], res.When)
	}
}

// intValue is n as an Int value.
func intValue(n int64) value.Value { return &value.Int{V: n, T: types.IntType} }
