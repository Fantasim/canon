package live_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/views"
	"github.com/fantasim/canonlang/internal/views/live"
	"github.com/fantasim/canonlang/internal/views/render"
)

// titles are the heading titles of res by key.
func titles(res *live.Result) map[string]live.Text {
	out := map[string]live.Text{}
	//canon:unordered a map copied into a map
	for k, h := range res.Headings {
		out[k] = h.Title
	}
	return out
}

// API.md V6, V9 (VIEWMODEL.md S9, 3.4): every element of every collection a field of the form
// holds gets a heading keyed by its relative path: equal titles disambiguated by position in a
// plain list, `index` and `key` given to the elements' titles.
func TestHeadings(t *testing.T) {
	_, res := sword(t, "")
	got := map[string]string{}
	//canon:unordered a map copied into a map
	for k, v := range titles(res) {
		got[k] = v.Value
	}
	same(t, "API.md V6 API.md V9", got, `{"parts[0]":"Blade (#1)","parts[1]":"Blade (#2)","parts[2]":"Hilt",
	 "rows[0]":"Rim","spares[left]":"Guard","steps[0]":"Step 1","steps[1]":"Step 2","slots[belt]":"Slot belt",
	 "marks[0]":"Mark (#1)","marks[1]":"Mark (#2)","markMap[m]":"Mark","notes[0]":"#1","plans[0]":"#1"}`)
}

// API.md V6 (VIEWMODEL.md S9; log-2026-09-29 M4 U9): equal titles are compared in the source
// language: French titles that collide while the source differs keep no suffix, French titles
// that differ while the source collides get one, a failing title gets none (V11).
func TestDisambiguationSource(t *testing.T) {
	p, res := sword(t, "fr")
	ores := evaluate(t, p, p.let(t, "a", "ores"), "ores", "fr")
	pool := evaluate(t, p, p.let(t, "a", "pool"), "pool", "fr")
	same(t, "API.md V6 S9", []any{titles(ores), titles(pool), res.Headings["marks[0]"].Title, res.Headings["markMap[m]"].Title}, `[
	 {"o1":{"Value":"Minerai","OK":true,"Fallback":false},"o2":{"Value":"Minerai","OK":true,"Fallback":false}},
	 {"[0]":{"Value":"Statistiques 1 (#1)","OK":true,"Fallback":false},"[1]":{"Value":"Statistiques 2 (#2)","OK":true,"Fallback":false}},
	 {"Value":"","OK":false,"Fallback":false}, {"Value":"Marque m","OK":true,"Fallback":false}]`)
}

// API.md V6 (VIEWMODEL.md Q4, S9, T4): when the value is a collection, each of its elements gets
// a heading, retired ones flagged, a ref in a title by its target's title (S8); elements deeper
// than that do not.
func TestCollectionHeadings(t *testing.T) {
	p := shopProject(t)
	res := evaluate(t, p, p.let(t, "a", "items"), "items", "")
	if len(res.Headings) != 2 || len(res.When) != 0 || len(res.Show) != 0 {
		t.Errorf("API.md V6: headings %v, when %v, show %v", res.Headings, res.When, res.Show)
	}
	same(t, "API.md V6 retired", []any{res.Headings["sword"].Title, res.Headings["bread"].Retired, res.Title, res.Subtitle}, `[
	 {"Value":"Item Sword","OK":true,"Fallback":false}, true, {"Value":"items","OK":true,"Fallback":false},
	 {"Value":"","OK":true,"Fallback":false}]`)
	metals := evaluate(t, p, p.let(t, "a", "metals"), "metals", "")
	same(t, "API.md V6 S8", metals.Headings["iron"].Title.Value, `"Metal Iron of Ore Rock"`)
}

// API.md V6a (VIEWMODEL.md T8, T9, S8): the cells of the `text` columns of a table, keyed by
// field key: a record's title, a dependent value's canonical text, a ref's target title (refs in
// it by key), a literal, an inline case field's record; a case field the row lacks is left out.
func TestCells(t *testing.T) {
	_, res := sword(t, "")
	same(t, "API.md V6a", []any{res.Headings["parts[0]"].Cells, res.Headings["parts[1]"].Cells}, `[
	 {"stats":{"Value":"Stats","OK":true,"Fallback":false},"target":{"Value":"4","OK":true,"Fallback":false},
	  "link":{"Value":"Metal Iron of o1","OK":true,"Fallback":false},"kind.weapon.grip":{"Value":"Stats","OK":true,"Fallback":false}},
	 {"stats":{"Value":"Stats","OK":true,"Fallback":false},"target":{"Value":"orc","OK":true,"Fallback":false},
	  "link":{"Value":"none","OK":true,"Fallback":false}}]`)
	if len(res.Headings["spares[left]"].Cells) != 0 || len(res.Headings["rows[0]"].Cells) != 0 {
		t.Errorf("API.md V6a: cards and a `text` hint have no cells: %v", res.Headings)
	}
}

// API.md V6a (VIEWMODEL.md C1): at a collection held by a field, the field's control hint holds:
// a `text` control has no cells, where the type alone would make a table.
func TestCollectionControl(t *testing.T) {
	p := shopProject(t)
	sw := p.item(t, "sword")
	rows := fieldValue(t, sw, "rows")
	decl := sw.T
	var f *types.Field
	for _, g := range decl.Base().(*types.RecordType).Fields {
		if g.Name == "rows" {
			f = g
		}
	}
	hinted := at(t, p.input(), live.Target{Value: rows, Name: "rows", Field: f, Decl: decl})
	bare := at(t, p.input(), live.Target{Value: rows, Name: "rows"})
	if len(hinted.Headings["[0]"].Cells) != 0 || len(bare.Headings["[0]"].Cells) == 0 {
		t.Errorf("API.md V6a: hinted %v, bare %v", hinted.Headings, bare.Headings)
	}
}

// API.md 11 `Types` (VIEWMODEL.md D6, J14, TYPES.md 11): each dependent field's type as its driver
// selects it: a field, down a record or ref, an enclosing or map-key parameter, a nested
// application; refinements of the arm and the field kept (log-2026-09-29 M4 U9).
func TestTypes(t *testing.T) {
	p, res := sword(t, "")
	str, num := `{"kind":"string"}`, `{"kind":"int","bits":64,"signed":true}`
	same(t, "API.md 11 Types", res.Types, `{"target":`+str+`,"bonus":`+str+`,"plan.aim":`+num+`,"wrapped":`+num+`,
	 "hint":{"kind":"string","predicate":"true"}}`)
	bread := evaluate(t, p, p.item(t, "bread"), "bread", "")
	same(t, "API.md 11 Types bread", bread.Types, `{"target":`+num+`,"bonus":`+num+`,"plan.aim":`+str+`,
	 "wrapped":{"kind":"list","of":`+str+`,"max":3},"hint":{"kind":"int","bits":64,"signed":true,"predicate":"true"}}`)
	plans := p.let(t, "a", "plans").(*value.Map)
	keyed := at(t, p.input(), live.Target{Value: plans.Vals[1], Magic: render.Magic{Key: plans.Keys[1]}})
	unkeyed := evaluate(t, p, plans.Vals[1], "m2", "")
	same(t, "API.md 11 Types map key", []any{keyed.Types, unkeyed.Types}, `[{"aim":`+num+`},{}]`)
}

// TYPES.md 11.1, API.md 11 `Types`: at an element or a nested record whose arguments its
// enclosing record binds, above the path, Input.Bound gives them; without it the entry is left
// out (log-2026-09-29 M4 U9).
func TestTypesBound(t *testing.T) {
	p := shopProject(t)
	sw := p.item(t, "sword")
	plan := fieldValue(t, sw, "plan").(*value.Record)
	element := fieldValue(t, sw, "plans").(*value.List).Elems[0].(*value.Record)
	param := plan.T.Base().(*types.AppliedRecord).Rec.Params[0]
	mode := fieldValue(t, sw, "mode")
	in := p.input()
	in.Bound = func(r *value.Record) map[*types.Param]value.Value {
		if r == plan || r == element {
			return map[*types.Param]value.Value{param: mode}
		}
		return nil
	}
	num := `{"aim":{"kind":"int","bits":64,"signed":true}}`
	got := []any{at(t, in, live.Target{Value: element}).Types, at(t, in, live.Target{Value: plan}).Types, evaluate(t, p, element, "", "").Types}
	same(t, "API.md 11 Types bound", got, `[`+num+`,`+num+`,{}]`)
}

// API.md V10 (VIEWMODEL.md G17, 12.4): the `_<n>` ids of unnamed `show` lines, across groups,
// are the view model's.
func TestShowIDs(t *testing.T) {
	p, res := sword(t, "")
	m, err := views.Build(context.Background(), views.Input{Program: p.a.Program(), Package: "a", Language: "0.1", I18N: p.texts})
	if err != nil {
		t.Fatal(err)
	}
	var want, got []string
	//canon:unordered sorted below
	for id := range m.Views["a.Item"].Shows {
		want = append(want, "Item.show."+id)
	}
	for _, l := range res.Show {
		if strings.HasPrefix(l.Key, "Item.show.") {
			got = append(got, l.Key)
		}
	}
	slices.Sort(want)
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Errorf("API.md V10: live %v, view model %v", got, want)
	}
}
