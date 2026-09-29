package i18n_test

import (
	"reflect"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/i18n"
)

// VIEWMODEL.md J4: a view holding an error is broken and left out of the catalogue; its
// members fall back to their default label (I18N.md L8, K "T.e": the Canon name).
func TestBrokenEnumViewFallsBackAndLeavesNoKey(t *testing.T) {
	src := map[string]string{"j4/j4.canon": `package j4

/// A palette.
enum Color {
  red
  blue
}

view Color {
  red "Red" { when: missing }
  blue "Blue"
}

emit view { out: "out/j4.view.json" }
`}
	c := checkFiles(t, src)
	bag := c.bags["j4"]
	if bag == nil {
		t.Fatal("no bag for package j4")
	}
	var errs int
	for _, f := range bag.Findings() {
		if f.Severity == diag.Error {
			errs++
		}
	}
	if errs == 0 {
		t.Fatal("want an error finding for the unresolved `missing`, got none")
	}
	res := c.res["j4"]
	if res == nil {
		t.Fatal("no i18n result for package j4")
	}
	cat := res.Catalogue
	for _, want := range []struct{ key, text string }{{"Color.red", "red"}, {"Color.blue", "blue"}} {
		e, ok := cat.Lookup(want.key)
		if !ok {
			t.Errorf("missing %s in %v", want.key, keys(cat))
			continue
		}
		if e.Text != want.text {
			t.Errorf("%s = %q, want the Canon name %q (the broken view's label dropped)", want.key, e.Text, want.text)
		}
	}
	for _, k := range keys(cat) {
		if k != "Color.help" && k != "Color.red" && k != "Color.blue" {
			t.Errorf("key %q survives the broken view, want none beyond the unconditional labels", k)
		}
	}

	// ADR-0009: check records BrokenViews when it reports the error, not read back from the bag.
	bag.Truncate(1)
	if got := len(bag.Findings()); got != 1 {
		t.Fatalf("Truncate(1): want 1 finding, got %d", got)
	}
	res2 := i18n.Check(c.prog, fixtureProject(), c.bags, emitsView(c.prog))["j4"]
	if res2 == nil {
		t.Fatal("no i18n result for package j4 (after Truncate(1))")
	}
	if !reflect.DeepEqual(cat.Entries, res2.Catalogue.Entries) {
		t.Errorf("catalogue changed after Truncate(1):\nbefore %v\nafter  %v", keys(cat), keys(res2.Catalogue))
	}
}
