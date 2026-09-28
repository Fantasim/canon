package i18n_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/i18n"
	"github.com/fantasim/canonlang/internal/syntax"
)

// I18N.md §3.3 `T.check.n`, K8, TYPES.md §12.1: a named one-line check of a variant outside its cases has the key `V.check.n`, which i18n and check both resolve; an unnamed one has none.
func TestCatalogueVariantLevelCheckKey(t *testing.T) {
	c := checkFiles(t, map[string]string{
		"vk/vk.canon": `package vk

/// A reward.
variant Reward {
  /// Some gold.
  gold {
    /// How much.
    amount: Int
  }
  /// Nothing.
  nothing

  check positive: self.kind == nothing or self is gold else "a {self.kind} reward"
  warn self.kind != nothing else "an empty reward"
}

emit view { out: "out/vk.view.json" }
`,
		"vk/vk.fr.canon": `package vk
translation fr

Reward.check.positive "une récompense {self.kind}"
`,
	})
	if out := c.render(t); strings.Contains(out, "error[") {
		t.Fatalf("the variant-level check's key must resolve:\n%s", out)
	}
	cat := c.res["vk"].Catalogue
	if !slices.Contains(keys(cat), "Reward.check.positive") {
		t.Errorf("no key Reward.check.positive in %v", keys(cat))
	}
	for _, e := range cat.Entries {
		if e.Kind == i18n.Template && e.Text == "an empty reward" {
			t.Errorf("an unnamed variant-level check should have no key, found %s", e.Key)
		}
	}
	for _, pkg := range c.prog.Packages {
		for _, f := range pkg.Files {
			if f.FileKind == syntax.FileTranslation {
				checkFileKeys(t, c.prog, f, cat)
			}
		}
	}
}
