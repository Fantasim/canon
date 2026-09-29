package encode

import (
	"strings"

	"github.com/fantasim/canonlang/api/vm"
	"github.com/fantasim/canonlang/internal/i18n"
	"github.com/fantasim/canonlang/internal/syntax"
)

// Texts writes text references from the key catalogues of the program's packages (VIEWMODEL.md
// J9, I18N.md 3): a key is written only when its package's catalogue holds it, so the source
// table always has it.
type Texts struct {
	cats map[string]*i18n.Catalogue
}

// NewTexts reads text references from cats, by package; a package without one has no key.
func NewTexts(cats map[string]*i18n.Catalogue) *Texts { return &Texts{cats: cats} }

// Text is the reference of the text keyed segs in pkg: its key when catalogued; the neutral
// text when the catalogue holds it without a letter (I18N.md L7); absent otherwise.
func (t *Texts) Text(pkg, text string, segs ...string) vm.TextRef {
	cat := t.catalogue(pkg)
	if cat == nil || text == "" {
		return vm.TextRef{}
	}
	key := strings.Join(segs, dot)
	switch res := cat.Resolve(key); {
	case res.Found:
		return vm.TextRef{Key: pkg + colon + key}
	case res.NoLetter:
		return vm.TextRef{Text: &text}
	}
	return vm.TextRef{}
}

// Key is the key segs of pkg when catalogued, absent otherwise (a template's `text`, 12.4).
func (t *Texts) Key(pkg string, segs ...string) vm.TextRef {
	if _, ok := t.Source(pkg, segs...); !ok {
		return vm.TextRef{}
	}
	return vm.TextRef{Key: pkg + colon + strings.Join(segs, dot)}
}

// Label is a text that always exists (a label, I18N.md L8), text its source: its key when
// catalogued, text when the catalogue holds it without a letter, else the Canon name as a
// neutral text (a studio symbol without a key, J9).
func (t *Texts) Label(pkg, text, name string, segs ...string) vm.TextRef {
	if ref := t.Text(pkg, text, segs...); ref != (vm.TextRef{}) {
		return ref
	}
	return vm.TextRef{Text: &name}
}

// Source is the source text of the key segs of pkg (I18N.md 3.3); false when not catalogued.
func (t *Texts) Source(pkg string, segs ...string) (string, bool) {
	cat := t.catalogue(pkg)
	if cat == nil {
		return "", false
	}
	e, ok := cat.Lookup(strings.Join(segs, dot))
	return e.Text, ok
}

func (t *Texts) catalogue(pkg string) *i18n.Catalogue {
	if t == nil {
		return nil
	}
	return t.cats[pkg]
}

// PlainText is a string literal without interpolation, unescaped; false for another expression.
func PlainText(e syntax.Expr) (string, bool) {
	switch x := e.(type) {
	case *syntax.RawStringLit:
		return x.Value, true
	case *syntax.StringLit:
		var b strings.Builder
		for _, p := range x.Parts {
			if p.Interp != nil {
				return "", false
			}
			b.WriteString(p.Text)
		}
		return b.String(), true
	}
	return "", false
}
