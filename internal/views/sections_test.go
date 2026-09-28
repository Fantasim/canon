package views_test

import (
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
)

// translated has French for one key and an empty French text for another (I18N.md F5).
var translated = map[string]string{
	"a/a.canon": `package a

/// A point.
record Pt {
  /// Across.
  x: Int
}
`,
	"a/a.fr.canon": "package a\ntranslation fr\n\nPt.x \"Abscisse\"\nPt.help \"\"\n",
}

// VIEWMODEL.md 12.10: the source language holds every catalogue key with its source text; each
// other language its translation files, the keys it misses and its non-empty translations.
func TestI18NSection(t *testing.T) {
	m := tree(t, "  languages: [en, fr]\n", translated, build.Options{}).model(t, demoPkg)
	expect(t, "12.10", m.I18N, `{"source":"en","languages":{
	 "en":{"texts":{"Pt.help":"A point.","Pt.x":"X","Pt.x.help":"Across."}},
	 "fr":{"files":["a/a.fr.canon"],"missing":2,"texts":{"Pt.x":"Abscisse"}}}}`)
}

// referring holds a package whose model refers to another package's type, collection and texts.
var referring = map[string]string{
	"a/a.canon": "package a\n\nrecord Item {\n  name: String\n}\n\nlet items: table Item = {\n  i1 { name: \"one\" }\n}\n",
	"b/b.canon": "package b\n\nimport a\n\nrecord Box {\n  item: a.Item\n  pick: ref a.items\n}\n",
}

// VIEWMODEL.md 12.1 `requires`, J7, J8: another package's type and collection are referenced,
// never copied, and the package holding them is required.
func TestRequires(t *testing.T) {
	m := tree(t, "", referring, build.Options{}).model(t, "b")
	expect(t, "12.1", m.Requires, `["a"]`)
	if _, copied := m.Types["a.Item"]; copied {
		t.Error("J7: a.Item is copied into b's types")
	}
}

// failing has a broken record, a value that fails to evaluate and a hidden required field.
const failing = `package a

record Broken {
  x: Nope
}

record Pt {
  x: Int
}

view Pt {
  x { hidden: true }
}

let bad: Int = [1][3]
`

// VIEWMODEL.md J4, J15: the model is built with errors: a broken declaration is left out, a
// value that failed is listed with failed; every finding of the package is in findings, in F2
// order.
func TestModelWithErrors(t *testing.T) {
	x := broken(t, "", map[string]string{"a/a.canon": failing}, build.Options{})
	m := x.model(t, demoPkg)
	if _, ok := m.Types["a.Broken"]; ok || m.Types["a.Pt"].Name != "Pt" {
		t.Errorf("J4: types %v", keysOf(m.Types))
	}
	if !m.Values["a:bad"].Failed {
		t.Error("J4: bad is not failed")
	}
	var codes, want []string
	for _, f := range m.Findings {
		codes = append(codes, f.Code)
	}
	for _, f := range x.bags[demoPkg].Findings() {
		want = append(want, string(f.Code))
	}
	if hidden := string(diag.W1641.Def().Code); !slices.Equal(codes, want) || !slices.Contains(codes, hidden) {
		t.Errorf("J15: findings %v, want %v with %s", codes, want, hidden)
	}
}
