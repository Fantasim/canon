package i18n_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/i18n"
)

// catalogue is pkg's catalogue from files, or nil when the package was not checked.
func catalogue(t *testing.T, files map[string]string, pkg string) *i18n.Catalogue {
	t.Helper()
	res := checkFiles(t, files).res[pkg]
	if res == nil {
		return nil
	}
	return res.Catalogue
}

func keys(cat *i18n.Catalogue) []string {
	out := make([]string, len(cat.Entries))
	for i, e := range cat.Entries {
		out[i] = e.Key
	}
	return out
}

// K1: every field of a record contributes, with no view at all.
func TestCatalogueFieldsUnconditional(t *testing.T) {
	cat := catalogue(t, map[string]string{"k1/k1.canon": `package k1

/// A thing.
record Thing {
  /// Its name.
  name: String
}
`}, "k1")
	for _, want := range []string{"Thing.help", "Thing.name", "Thing.name.help"} {
		e, ok := cat.Lookup(want)
		if !ok {
			t.Errorf("missing %s in %v", want, keys(cat))
			continue
		}
		if e.Kind != i18n.Plain {
			t.Errorf("%s: Kind = %v, want Plain", want, e.Kind)
		}
	}
}

// K1: a local type reached from a public let (no code emit, TYPES.md §3.7) through a field contributes; an orphan does not.
func TestCatalogueReachability(t *testing.T) {
	cat := catalogue(t, map[string]string{"k1r/k1r.canon": `package k1r

/// Reached through Outer.
local record Inner {
  /// Its value.
  value: Int
}

/// Never used.
local record Orphan {
  /// Dead.
  dead: Int
}

/// Holds the inner thing.
local record Outer {
  /// The inner thing.
  inner: Inner
}

/// The public root.
let root: Outer = { inner: { value: 1 } }
`}, "k1r")
	if _, ok := cat.Lookup("Inner.help"); !ok {
		t.Errorf("Inner should be reachable: %v", keys(cat))
	}
	if _, ok := cat.Lookup("Orphan.help"); ok {
		t.Errorf("Orphan is unreachable and should not contribute: %v", keys(cat))
	}
}

// K1: an applied record (`inner: Inner(k)`) and a dependent type function's arm record both
// contribute (VIEWMODEL.md's dependent types, reachable like any other type expression).
func TestCatalogueDependentTypesReachable(t *testing.T) {
	cat := catalogue(t, map[string]string{"k1d/k1d.canon": `package k1d

/// A kind.
enum Kind { a, b }

/// A parameterized record.
record Inner(k: Kind) {
  /// Its value.
  value: Int
}

/// Only reached through an arm of Target.
record KillP {
  /// Its detail.
  detail: String
}

/// Picks a type by goal.
type Target(g: Kind) = match g {
  a => KillP
  b => Int
}

/// The outer record.
record Outer {
  /// Chooses.
  goal: Kind
  /// An applied record.
  inner: Inner(goal)
  /// A dependent type application.
  target: Target(goal)
}
`}, "k1d")
	for _, key := range []string{"Inner.help", "Inner.value", "KillP.help", "KillP.detail"} {
		if _, ok := cat.Lookup(key); !ok {
			t.Errorf("missing %s in %v", key, keys(cat))
		}
	}
}

// K1: a ref's target element type is reachable too.
func TestCatalogueRefTargetReachable(t *testing.T) {
	cat := catalogue(t, map[string]string{"k1ref/k1ref.canon": `package k1ref

/// A thing, reached only through a ref.
local record Thing {
  /// Its id.
  id: String
}

local let things: [Thing] keyed by id = []

/// The outer record.
record Outer {
  /// A reference into things.
  thing: ref things
}
`}, "k1ref")
	for _, key := range []string{"Thing.help", "Thing.id", "Thing.id.help"} {
		if _, ok := cat.Lookup(key); !ok {
			t.Errorf("missing %s in %v", key, keys(cat))
		}
	}
}

// K5: the first segment of a key is always a name; a public top-level `let title` is not
// treated as the reserved segment "title" (its key is bare `title`, not `field.title`), and a
// translation `title "…"` matches it.
func TestCatalogueLetNamedReservedWord(t *testing.T) {
	src := map[string]string{
		"k5/k5.canon": `package k5

/// A menu label.
let title: String = "Menu label"
`,
		"k5/k5.fr.canon": `package k5
translation fr

title "Le titre"
title.help "L'aide"
`,
	}
	c := checkFiles(t, src)
	res := c.res["k5"]
	if res == nil {
		t.Fatal("no result for k5")
	}
	for _, key := range []string{"title", "title.help"} {
		if _, ok := res.Catalogue.Lookup(key); !ok {
			t.Errorf("missing %s in %v", key, keys(res.Catalogue))
		}
	}
	if out := c.render(t); out != "0 errors, 0 warnings in 1 package (…)\n" {
		t.Errorf("title and title.help should resolve with no finding:\n%s", out)
	}
	if got := res.Languages["fr"].Texts["title"]; got != "Le titre" {
		t.Errorf("fr title = %q, want %q", got, "Le titre")
	}
}

// I18N.md K "T.show.s", VIEWMODEL.md G17: an unnamed show line's key is `_<n>`, its 0-based
// position among the view's unnamed show lines.
func TestCatalogueUnnamedShowID(t *testing.T) {
	cat := catalogue(t, map[string]string{"tshow/tshow.canon": `package tshow

/// A thing.
record Thing {
  /// Its name.
  name: String
}

view Thing {
  show "First" "First {name}"
  show "Second" "Second {name}"
}
`}, "tshow")
	for _, key := range []string{"Thing.show._0", "Thing.show._0.text", "Thing.show._1", "Thing.show._1.text"} {
		if _, ok := cat.Lookup(key); !ok {
			t.Errorf("missing %s in %v", key, keys(cat))
		}
	}
}

// I18N.md K "T.f.none", "T.f.step": a field's `none` and `step` view props each get their own
// key (MOCKUP-GAPS 12; `step`'s is a template).
func TestCatalogueFieldNoneAndStep(t *testing.T) {
	cat := catalogue(t, map[string]string{"tprops/tprops.canon": `package tprops

/// A thing.
record Thing {
  /// Its level.
  level: Int? = none
}

view Thing {
  level "Level" { none: "Unset", step: "Level {index}" }
}
`}, "tprops")
	none, ok := cat.Lookup("Thing.level.none")
	if !ok || none.Text != "Unset" {
		t.Errorf("Thing.level.none = %+v, ok=%v, want text %q", none, ok, "Unset")
	}
	step, ok := cat.Lookup("Thing.level.step")
	if !ok || step.Text != "Level {index}" || step.Kind != i18n.Template {
		t.Errorf("Thing.level.step = %+v, ok=%v, want template %q", step, ok, "Level {index}")
	}
}

// I18N.md F3: a package's translation files for one language are merged.
func TestTranslationFilesMergeAcrossTwoFiles(t *testing.T) {
	c := checkFiles(t, map[string]string{
		"f3/f3.canon": `package f3

/// A thing.
record Thing {
  /// Its a.
  a: String
  /// Its b.
  b: String
}

emit view { out: "out/f3.view.json" }
`,
		"f3/f3.a.fr.canon": `package f3
translation fr

Thing.a "A fr"
`,
		"f3/f3.b.fr.canon": `package f3
translation fr

Thing.b "B fr"
`,
	})
	res := c.res["f3"]
	if res == nil {
		t.Fatal("no result for f3")
	}
	if out := c.render(t); strings.Contains(out, "error[") {
		t.Errorf("two merged translation files' known keys should report no error:\n%s", out)
	}
	fr := res.Languages["fr"]
	if fr.Texts["Thing.a"] != "A fr" || fr.Texts["Thing.b"] != "B fr" {
		t.Errorf("Texts = %+v, want both files merged", fr.Texts)
	}
	if len(fr.Files) != 2 {
		t.Errorf("Files = %v, want both files listed", fr.Files)
	}
}

// K2: a method contributes only when a view names it.
func TestCatalogueMethodOnlyWithView(t *testing.T) {
	src := map[string]string{"k2/k2.canon": `package k2

/// A thing.
record Thing {
  /// Its name.
  name: String

  export fn label(self) -> String { return name }
}

view Thing {
  label "Label"
}
`}
	cat := catalogue(t, src, "k2")
	if _, ok := cat.Lookup("Thing.label"); !ok {
		t.Errorf("a method named by a view should contribute: %v", keys(cat))
	}

	unviewed := map[string]string{"k2b/k2b.canon": `package k2b

/// A thing.
record Thing {
  /// Its name.
  name: String

  export fn label(self) -> String { return name }
}
`}
	cat2 := catalogue(t, unviewed, "k2b")
	if _, ok := cat2.Lookup("Thing.label"); ok {
		t.Errorf("a method no view names should not contribute: %v", keys(cat2))
	}
}

// K4: a field whose name is a reserved segment is written with its kind word; the bare form
// names nothing.
func TestCatalogueReservedSegmentForm(t *testing.T) {
	cat := catalogue(t, map[string]string{"k4/k4.canon": `package k4

/// A widget.
record Widget {
  /// The literal word "title".
  title: String
}
`}, "k4")
	if _, ok := cat.Lookup("Widget.field.title"); !ok {
		t.Errorf("a field named title should be Widget.field.title: %v", keys(cat))
	}
	if _, ok := cat.Lookup("Widget.title"); ok {
		t.Errorf("the bare form should not exist for a reserved name: %v", keys(cat))
	}
	if got := cat.Resolve("Widget.title").FormHint; got != "Widget.field.title" {
		t.Errorf("FormHint = %q, want Widget.field.title", got)
	}
}

// K8: an unnamed check has no key.
func TestCatalogueUnnamedCheckNoKey(t *testing.T) {
	cat := catalogue(t, map[string]string{"k8/k8.canon": `package k8

/// A thing.
record Thing {
  /// Its value.
  value: Int

  warn value != 0 else "a value of 0 has no effect"
}
`}, "k8")
	for _, e := range cat.Entries {
		if e.Kind == i18n.Template && e.Text == "a value of 0 has no effect" {
			t.Errorf("an unnamed check should have no key, found %s", e.Key)
		}
	}
}

// K8: a block-form check (necessarily unnamed, GRAMMAR.md "check {" always starts the block
// form) has no key either.
func TestCatalogueBlockCheckNoKey(t *testing.T) {
	cat := catalogue(t, map[string]string{"k8b/k8b.canon": `package k8b

/// A thing.
record Thing {
  /// Its value.
  value: Int

  check {
    fail(value, "a block check message with a letter")
  }
}
`}, "k8b")
	for _, e := range cat.Entries {
		if e.Kind == i18n.Template && e.Text == "a block check message with a letter" {
			t.Errorf("a block-form check should have no key, found %s", e.Key)
		}
	}
}

// K9: a retired enum member and a deprecated field keep their keys.
func TestCatalogueRetiredAndDeprecatedKeepKeys(t *testing.T) {
	cat := catalogue(t, map[string]string{"k9/k9.canon": `package k9

/// Kinds.
enum Kind { active, retired legacy }

/// A thing.
record Thing {
  /// Its cap.
  cap: Int? = none
    @deprecated("no longer read")
}
`}, "k9")
	if _, ok := cat.Lookup("Kind.legacy"); !ok {
		t.Errorf("a retired member should keep its key: %v", keys(cat))
	}
	if _, ok := cat.Lookup("Thing.cap.deprecated"); !ok {
		t.Errorf("a deprecated field should keep its keys: %v", keys(cat))
	}
}

// K10: catalogue order is the byte order of the keys.
func TestCatalogueByteOrder(t *testing.T) {
	cat := catalogue(t, map[string]string{"k10/k10.canon": `package k10

/// A widget.
record Widget {
  /// Zeta.
  zeta: String
  /// Alpha.
  alpha: String
}
`}, "k10")
	got := keys(cat)
	sorted := slices.Clone(got)
	slices.Sort(sorted)
	if !slices.Equal(got, sorted) {
		t.Errorf("catalogue order = %v, want byte order %v", got, sorted)
	}
}

// I18N.md K3: the studio package's catalogue holds only Menu.<member> and units.<unit>.suffix.
func TestCatalogueStudioVocabulary(t *testing.T) {
	cat := catalogue(t, map[string]string{"studio/studio.canon": `package studio

/// Top-level sections.
enum Menu {
  /// Seasonal and limited events.
  events
  economy
}

/// How numbers are shown.
record UnitSpec {
  /// Appended to the number.
  suffix: String = ""
}

let units: table UnitSpec = {
  penya { suffix: " penya" }
  pct { suffix: " %" }
}
`}, "studio")
	want := []string{"Menu.events", "Menu.economy", "Menu.events.help", "units.penya.suffix"}
	for _, k := range want {
		if _, ok := cat.Lookup(k); !ok {
			t.Errorf("missing %s in %v", k, keys(cat))
		}
	}
	if _, ok := cat.Lookup("units.pct.suffix"); ok {
		t.Errorf("a suffix with no letter should not contribute: %v", keys(cat))
	}
	if _, ok := cat.Lookup("UnitSpec.help"); ok {
		t.Errorf("the studio package's other declarations should contribute nothing: %v", keys(cat))
	}
}
