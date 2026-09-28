package i18n_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/i18n"
)

// I18N.md B1: the source text is used wherever lang has no non-empty translation for a key.
func TestFallback(t *testing.T) {
	src := map[string]string{
		"fb/fb.canon": `package fb

/// A thing.
record Thing {
  /// Its name.
  name: String
}

emit view { out: "out/thing.view.json" }
`,
		"fb/fb.fr.canon": `package fb
translation fr

Thing.help "Une chose."
`,
	}
	res := checkFiles(t, src).res["fb"]
	fr := res.Languages["fr"]

	if got := i18n.Fallback(res.Catalogue, fr, "Thing.help"); got.Value != "Une chose." || got.Fallback {
		t.Errorf("translated key: got %+v", got)
	}
	if got := i18n.Fallback(res.Catalogue, fr, "Thing.name"); got.Value != "Name" || !got.Fallback {
		t.Errorf("missing translation should fall back to the source text: got %+v", got)
	}
	if got := i18n.Fallback(res.Catalogue, nil, "Thing.name"); got.Value != "Name" || got.Fallback {
		t.Errorf("the source language itself never falls back: got %+v", got)
	}
	if got := i18n.Fallback(res.Catalogue, fr, "Nope.nope"); got != (i18n.Text{}) {
		t.Errorf("an unknown key gives the zero Text: got %+v", got)
	}
}
