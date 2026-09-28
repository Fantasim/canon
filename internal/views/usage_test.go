package views_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/build"
)

// shaped holds records with an inline variant and a variant field, a type of another package,
// and a keyed list that fails to evaluate.
var shaped = map[string]string{
	"a/a.canon": "package a\n\nrecord Ext {\n  n: Int\n}\n",
	"b/b.canon": `package b

import a

variant Kind {
  weapon { attack: Int }
  food { heal: Int = 0 }
}

variant Shape {
  circle { r: Int }
  square { side: Int }
}

record Item {
  kind: Kind @json(inline)
  shape: Shape
  ext: a.Ext
}

record Row {
  x: Int
}

let items: [Item] = [
  { kind: weapon { attack: 1 }, shape: circle { r: 1 }, ext: { n: 1 } },
  { kind: food {}, shape: square { side: 2 }, ext: { n: 2 } },
]

local let rows: [Row] keyed by x = [{ x: 1 }]

let lost: [Row] keyed by x = [rows][2]
`,
}

// VIEWMODEL.md L7, L10, L17, L19, 12.7, J4: shape keys name inline cases (their fields keyed
// under them) or a variant's case; another package's type counts under its qualified name; a
// failed value gives no usage and no index rows.
func TestUsageShapes(t *testing.T) {
	m := broken(t, "", shaped, build.Options{}).model(t, "b")
	expect(t, "L17 L19", m.Usage["b.Item"], `{"shapes":{
	 "kind=food":{"count":1,"fields":{"ext":1,"kind":1,"kind.food.heal":0,"shape":1},"main":["kind","shape","ext"],"more":["kind.food.heal"]},
	 "kind=weapon":{"count":1,"fields":{"ext":1,"kind":1,"kind.weapon.attack":1,"shape":1},"main":["kind","kind.weapon.attack","shape","ext"]}}}`)
	expect(t, "12.7 variant", m.Usage["b.Shape"], `{"shapes":{"circle":{"count":1,"fields":{"r":1},"main":["r"]},
	 "square":{"count":1,"fields":{"side":1},"main":["side"]}}}`)
	expect(t, "L10", m.Usage["a.Ext"], `{"shapes":{"":{"count":2,"fields":{"n":2},"main":["n"]}}}`)
	if _, ok := m.Usage["b.Row"]; ok || !m.Values["b:lost"].Failed {
		t.Errorf("J4: usage of a failed value %v, failed %v", m.Usage["b.Row"], m.Values["b:lost"].Failed)
	}
	expect(t, "J4 S2", m.Search["b:lost"].Rows, `[]`)
}
