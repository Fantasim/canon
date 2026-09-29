package live_test

import (
	"strconv"
	"testing"

	"github.com/fantasim/canonlang/internal/value"
)

// API.md V6-V9 over examples/resource/farm, in French: the farm's translated title; its models
// headed by `modelTypes[<typeId>]`, `{typeName}` having no key; a model's levels by `levels[<n>]`
// with the French template `Palier {level}`.
func TestExampleFarm(t *testing.T) {
	p := examples(t)
	farm := p.let(t, "resource.farm", "farm")
	res := evaluate(t, p, farm, "farm", "fr")
	same(t, "API.md V8 title", res.Title, `{"Value":"Ferme","OK":true,"Fallback":false}`)
	models := fieldValue(t, farm, "modelTypes").(*value.List)
	first := models.Elems[0].(*value.Record)
	key := "modelTypes[" + first.Ident.Key.Text() + "]"
	name := fieldValue(t, first, "typeName").CanonText()
	same(t, "API.md V6 API.md V9", res.Headings[key].Title, `{"Value":"`+name+`","OK":true,"Fallback":false}`)
	if len(res.Headings) != len(models.Elems) {
		t.Errorf("API.md V6: %d headings for %d models", len(res.Headings), len(models.Elems))
	}
	model := evaluate(t, p, first, first.Ident.Key.Text(), "fr")
	level := fieldValue(t, fieldValue(t, first, "levels").(*value.List).Elems[0], "level").CanonText()
	same(t, "API.md V8 V9 levels", model.Headings["levels[0]"].Title, `{"Value":"Palier `+level+`","OK":true,"Fallback":false}`)
}

// API.md 11 `Types` over examples/resource/heistia (VIEWMODEL.md 13): a task's filterParam is
// the branch of Param its event type's `param` selects.
func TestExampleHeistiaTypes(t *testing.T) {
	p := examples(t)
	tasks := fieldValue(t, p.let(t, "resource.heistia", "heistia"), "tasks").(*value.List)
	n := strconv.Itoa(len(p.let(t, "resource.vocab", "items").(*value.List).Elems))
	for _, el := range tasks.Elems {
		if fieldValue(t, el, "eventType").CanonText() != "ECONOMY_DROP_ITEM" {
			continue
		}
		res := evaluate(t, p, el, "", "")
		same(t, "API.md 11 Types", res.Types, `{"filterParam":{"kind":"ref","collection":"resource.vocab:items",
		 "element":"resource.vocab.Item","keyType":"string","count":`+n+`,"active":`+n+`}}`)
		return
	}
	t.Fatal("no ECONOMY_DROP_ITEM task")
}

// API.md V6, V7 over examples/resource/vocab: an item is headed by its key, its preview is its
// icon file and its subtitle its `{id}`.
func TestExampleVocabPreview(t *testing.T) {
	p := examples(t)
	items := p.let(t, "resource.vocab", "items").(*value.List)
	first := items.Elems[0].(*value.Record)
	res := evaluate(t, p, items, "items", "")
	h := res.Headings["["+first.Ident.Key.Text()+"]"]
	icon := fieldValue(t, first, "icon").CanonText()
	same(t, "API.md V6 API.md V7", []any{h.Subtitle.Value, h.Preview}, `["`+first.Ident.Key.Text()+`","`+icon+`"]`)
}
