package i18n_test

import "testing"

// K7: a case field with no view of its own is labelled through the outer record's view that
// inlines its variant, across every case that has a matching field.
func TestCatalogueInlineCaseFieldLabel(t *testing.T) {
	cat := catalogue(t, map[string]string{"k7/k7.canon": `package k7

/// A kind of thing.
variant Kind {
  Foo {
    /// Foo's own name.
    name: String
  }
  Bar {
    /// Bar's own name.
    name: String
  }
}

/// The outer record.
record Item {
  /// What it is.
  kind: Kind @json(inline)
}

view Item {
  name "Common name"
}
`}, "k7")
	for _, key := range []string{"Kind.Foo.name", "Kind.Bar.name"} {
		e, ok := cat.Lookup(key)
		if !ok {
			t.Fatalf("missing %s in %v", key, keys(cat))
		}
		if e.Text != "Common name" {
			t.Errorf("%s: Text = %q, want %q (I18N.md K7)", key, e.Text, "Common name")
		}
	}
}

// K7, I18N.md L8, VIEWMODEL.md J4: a broken parent view supplies no inline case-field label
// either; the field falls back to its own default (the Canon name, humanized).
func TestCatalogueInlineParentViewBrokenFallsBack(t *testing.T) {
	cat := catalogue(t, map[string]string{"k7b/k7b.canon": `package k7b

/// Kind.
variant Kind {
  /// Sword.
  sword {
    /// Damage.
    damage: Int
  }
  /// Shield.
  shield {
    /// Armor.
    armor: Int
  }
}

/// Item.
record Item {
  /// Name.
  name: String
  /// Kind.
  kind: Kind @json(inline)
}

view Item {
  title "Item {missing}"
  damage "Dégâts infligés"
}
`}, "k7b")
	e, ok := cat.Lookup("Kind.sword.damage")
	if !ok {
		t.Fatalf("missing Kind.sword.damage in %v", keys(cat))
	}
	if e.Text != "Damage" {
		t.Errorf("Kind.sword.damage: Text = %q, want %q (Item's broken view supplies nothing, K7)", e.Text, "Damage")
	}
}

// K7: a parent view in another package supplies nothing; kind's own catalogue keeps its
// default labels even though item's view (in a different package) names "name" too.
func TestCatalogueInlineOnlySamePackage(t *testing.T) {
	cat := catalogue(t, map[string]string{
		"kind/kind.canon": `package kind

/// A kind of thing.
variant Kind {
  Foo {
    /// Foo's own name.
    name: String
  }
  Bar {
    /// Bar's own name.
    name: String
  }
}
`,
		"item/item.canon": `package item

import kind { Kind }

/// The outer record, in another package.
record Item {
  /// What it is.
  kind: Kind @json(inline)
}

view Item {
  name "Common name"
}
`,
	}, "kind")
	for _, key := range []string{"Kind.Foo.name", "Kind.Bar.name"} {
		e, ok := cat.Lookup(key)
		if !ok {
			t.Fatalf("missing %s in %v", key, keys(cat))
		}
		if e.Text != "Name" {
			t.Errorf("%s: Text = %q, want the default %q, not item's view (I18N.md K7 same package)", key, e.Text, "Name")
		}
	}
}

// K2: a variant case's own method contributes only when the case's own view names it.
func TestCatalogueCaseMethodOnlyWithView(t *testing.T) {
	src := map[string]string{"k2c/k2c.canon": `package k2c

/// A shape.
variant Shape {
  Circle {
    /// Its radius.
    radius: Int

    export fn area(self) -> Int { return radius * radius }
  }
}

view Shape.Circle {
  area "Area"
}
`}
	cat := catalogue(t, src, "k2c")
	if _, ok := cat.Lookup("Shape.Circle.area"); !ok {
		t.Errorf("a case method named by its own view should contribute: %v", keys(cat))
	}

	unviewed := map[string]string{"k2cb/k2cb.canon": `package k2cb

/// A shape.
variant Shape {
  Circle {
    /// Its radius.
    radius: Int

    export fn area(self) -> Int { return radius * radius }
  }
}
`}
	cat2 := catalogue(t, unviewed, "k2cb")
	if _, ok := cat2.Lookup("Shape.Circle.area"); ok {
		t.Errorf("a case method no view names should not contribute: %v", keys(cat2))
	}
}

// I18N.md K "T.m.help": a method's help falls back to its own doc comment when its view gives
// no `help` prop.
func TestCatalogueMethodHelpFallsBackToDoc(t *testing.T) {
	cat := catalogue(t, map[string]string{"tm/tm.canon": `package tm

/// A thing.
record Thing {
  /// Its name.
  name: String

  /// What it computes.
  export fn label(self) -> String { return name }
}

view Thing {
  label "Label"
}
`}, "tm")
	e, ok := cat.Lookup("Thing.label.help")
	if !ok {
		t.Fatalf("missing Thing.label.help in %v", keys(cat))
	}
	if e.Text != "What it computes." {
		t.Errorf("Thing.label.help = %q, want the method's doc comment", e.Text)
	}
}
