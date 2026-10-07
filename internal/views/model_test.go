package views_test

import (
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
)

// VIEWMODEL.md 12.9 `studio`, V2: a studio without a sound `Menu` enum (here a record) has no
// `menus`, and its model still validates.
func TestStudioWithoutMenu(t *testing.T) {
	files := map[string]string{"studio/studio.canon": `/// The studio.
package studio

/// Not an enum.
record Menu {
  /// A field.
  x: Int
}

/// Icons.
enum Icon { gem }

/// Tones.
enum Tone { info }
`}
	m := tree(t, "", files, build.Options{}).model(t, studioPkg)
	if m.Studio == nil || m.Studio.Menus != "" {
		t.Fatalf("12.9: studio %+v", m.Studio)
	}
	for _, e := range wholeSchema(t).Validate(written(t, m)) {
		t.Errorf("V2: %v", e)
	}
}

// VIEWMODEL.md S1, J12: a local table a ref targets only through a record's parameter type is
// indexed, so the `collection` the ref names has its index.
func TestRefTargetThroughParameter(t *testing.T) {
	src := `package a

record T {
  x: Int
}

local let ts: table T = {
  t1 { x: 1 }
}

record P(c: ref ts) {
  y: Int
}
`
	if _, ok := demo(t, src, "").model(t, demoPkg).Search["a:ts"]; !ok {
		t.Error("S1: a:ts, targeted through a parameter, has no index")
	}
}

// VIEWMODEL.md T6a, T11: `filters { kind }` in a variant view is the filter on the case, field
// `$case`, a choice over the cases (checkboxes with `multi`), and no E1606. Assumed: check
// records `kind` as the built-in (ObjBuiltin, check follow-ups 4) or leaves it unresolved.
func TestCaseFilter(t *testing.T) {
	src := `package a

variant Shape {
  circle { r: Int }
  square { r: Int }
}

view Shape {
  filters { kind multi, r }
}

let shapes: [Shape] = [circle { r: 1 }]
`
	m := broken(t, "", map[string]string{"a/a.canon": src}, build.Options{}).model(t, demoPkg)
	expect(t, "T6a", m.Values["a:shapes"].Control.Filters, `[
	 {"field":"$case","kind":"case","multi":true,"control":{"kind":"checkboxes","source":{"cases":"a.Shape"}}},
	 {"field":"circle.r","kind":"range","control":{"kind":"range","element":{"kind":"number"}}},
	 {"field":"square.r","kind":"range","control":{"kind":"range","element":{"kind":"number"}}}]`)
	for _, f := range m.Findings {
		if f.Code == string(diag.E1606.Def().Code) {
			t.Errorf("T6a: %s %s", f.Code, f.Message)
		}
	}
}

// VIEWMODEL.md 12.5 `singular`, T3: a list and cards control carry their element view's
// `singular`.
func TestSingularOnListAndCards(t *testing.T) {
	src := `package a

record R {
  x: Int
}

view R {
  singular "a thing"
}

record H {
  byName: {String: R}
  maybe: [R?]
}
`
	f := demo(t, src, "").model(t, demoPkg).Views["a.H"].Fields
	got := []string{f["byName"].Control.Kind, f["byName"].Control.Singular.Key, f["maybe"].Control.Kind, f["maybe"].Control.Singular.Key}
	if want := []string{"cards", "a:R.singular", "list", "a:R.singular"}; !slices.Equal(got, want) {
		t.Errorf("12.5: %v, want %v", got, want)
	}
}

// VIEWMODEL.md X4: `none` from an interpolation that reads no field renders as `none`.
func TestNoneFromAnExpression(t *testing.T) {
	src := `package a

record N {
  size: Int = 1
}

view N {
  title "N {size + 1}"
}

let ns: table N = {
  n1 {}
}
`
	x := demo(t, src, "")
	rows := x.modelWith(t, demoPkg, noneElse{standIn{info: x.a.Program().Info}}).Search["a:ns"].Rows
	expect(t, "X4", rows, `[{"key":"n1","title":"N none"}]`)
}

// VIEWMODEL.md J4, D5: a view holding an error (here E4503, a `+` format on a `String?`) is
// broken: its target stays in `types`, its view is left out of `views` and its templates are not
// rendered; a broken `view V.c` removes only itself, the case laid out by default.
func TestBrokenViewLeftOut(t *testing.T) {
	src := `package a

record N {
  note: String?
  x: Int
}

view N {
  title "N {note:+}"
}

let ns: table N = {
  n1 { x: 1 }
}

variant K {
  a { note: String? }
  b { x: Int }
}

view K {
  title "K"
}

view K.a {
  title "A {note:+}"
}

view K.b {
  title "B"
}
`
	m := broken(t, "", map[string]string{"a/a.canon": src}, build.Options{}).model(t, demoPkg)
	if _, ok := m.Views["a.N"]; ok || m.Types["a.N"].Name != "N" {
		t.Errorf("J4: views %v, types %v", keysOf(m.Views), keysOf(m.Types))
	}
	expect(t, "J4", m.Search["a:ns"].Rows, `[{"key":"n1"}]`)
	k := m.Views["a.K"]
	if !k.Declared || k.Cases["a"].Declared || k.Cases["a"].Title != nil || !k.Cases["b"].Declared {
		t.Errorf("J4 D5: K declared %v, a %v %v, b %v", k.Declared, k.Cases["a"].Declared, k.Cases["a"].Title, k.Cases["b"].Declared)
	}
}

// VIEWMODEL.md J4, S8: a view broken in another package renders in no model: a ref to its
// entries renders their key.
func TestBrokenViewAcrossPackages(t *testing.T) {
	files := map[string]string{
		"a/a.canon": `package a

record Tag {
  note: String?
}

view Tag {
  title "T {note:+}"
}

let tags: table Tag = {
  t1 {}
}
`,
		"b/b.canon": `package b

import a

record H {
  t: ref a.tags
}

view H {
  title "H {t}"
}

let hs: table H = {
  h1 { t: t1 }
}
`,
	}
	m := broken(t, "", files, build.Options{}).model(t, "b")
	expect(t, "J4 S8", m.Search["b:hs"].Rows, `[{"key":"h1","title":"H t1"}]`)
}

// A view whose only error is the views package's own (an unknown menu) still renders (ADR-0009).
func TestE16xxAloneDoesNotBreakView(t *testing.T) {
	files := map[string]string{
		"studio/studio.canon": `/// The studio.
package studio

/// Menus.
enum Menu { items }

/// Icons.
enum Icon { gem }

/// Tones.
enum Tone { info }
`,
		"a/a.canon": `package a

/// An item.
record Item {
  /// Its label.
  label: String
}

view Item {
  menu store icon gem
}

let items: table Item = {
  one { label: "x" }
}
`,
	}
	x := broken(t, "", files, build.Options{})
	m := x.model(t, "a")
	view, ok := m.Views["a.Item"]
	if !ok {
		t.Fatal("a.Item: want it still in views (the unknown menu alone does not break it)")
	}
	if _, ok := view.Fields["label"]; !ok {
		t.Error("a.Item.label: want it still rendered")
	}
	if code := diag.E1610.Def().Code; !hasFindingCode(x.bags["a"], code) {
		t.Errorf("want %s reported for the unknown menu (its own error, not swallowed)", code)
	}
}

// hasFindingCode reports bag holding a finding of code.
func hasFindingCode(bag *diag.Bag, code diag.Code) bool {
	if bag == nil {
		return false
	}
	for _, f := range bag.Findings() {
		if f.Code == code {
			return true
		}
	}
	return false
}

// VIEWMODEL.md 12.3 `asset.root`, 12.9 `assets`, WIRE.md 2.3: an unrooted root is resolved from
// the file declaring its type and keyed by its display path, so one root written in two
// directories is two roots.
func TestAssetRootsByDisplayPath(t *testing.T) {
	files := map[string]string{
		"a/a.canon":     "package a\n\nrecord Top {\n  icon: asset(\"icons\", ext: [png])\n}\n",
		"a/sub/b.canon": "package a\n\nrecord Deep {\n  icon: asset(\"icons\", ext: [png])\n}\n",
	}
	m := tree(t, "", files, build.Options{}).model(t, demoPkg)
	expect(t, "12.3", []any{m.Types["a.Top"].Fields[0].Type.Root, m.Types["a.Deep"].Fields[0].Type.Root}, `["a/icons","a/sub/icons"]`)
	expect(t, "12.9", m.Assets, `{"a/icons":{"dir":"a/icons"},"a/sub/icons":{"dir":"a/sub/icons"}}`)
	expect(t, "C14", m.Views["a.Deep"].Fields["icon"].Control.Root, `"a/sub/icons"`)
}

// VIEWMODEL.md 12.9, DECISIONS 108, 332: `assets.<root>.dir` is placed through project.canon's roots
// alone, so --root (Options.Roots) never changes it: the view model is the same on every machine.
func TestAssetDirIgnoresRootOverride(t *testing.T) {
	files := map[string]string{"a/a.canon": "package a\n\nrecord Top {\n  icon: asset(\"@res/Icon\", ext: [png])\n}\n"}
	head := "  roots {\n    res: \"../Resource\"\n  }\n"
	want := `{"@res/Icon":{"dir":"../Resource/Icon"}}`
	plain := tree(t, head, files, build.Options{}).model(t, demoPkg)
	moved := tree(t, head, files, build.Options{Roots: map[string]string{"res": "elsewhere/Res"}}).model(t, demoPkg)
	expect(t, "12.9 declared", plain.Assets, want)
	expect(t, "12.9 --root", moved.Assets, want)
}
