package render

import (
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
)

// Template is a view template, keyed Key in Pkg (I18N.md 3.3), and its source text.
type Template struct {
	Pkg    string
	Key    []string
	Source syntax.StrLit
}

// Template renders t for e in lang, translated when it can be (X6); false when it fails (X7).
func (r *Renderer) Template(t Template, e *value.Record, lang string) (string, bool) {
	src := t.Source
	if s, ok := r.Translated(t.Pkg, lang, t.Key); ok {
		src = s
	}
	return r.template(src, e, r.magicOf(e), lang)
}
