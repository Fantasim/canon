package encode

import (
	"strings"

	"github.com/fantasim/canonlang/api/vm"
	"github.com/fantasim/canonlang/internal/syntax"
)

// TemplateSource is a template's source text in the form of translation files (VIEWMODEL.md
// 12.4 `template`, I18N.md F6): literal braces doubled, each interpolation as written.
func TemplateSource(f *syntax.File, s syntax.StrLit) string {
	var b strings.Builder
	switch x := s.(type) {
	case *syntax.RawStringLit:
		doubled(&b, x.Value)
	case *syntax.StringLit:
		for _, p := range x.Parts {
			if p.Interp == nil {
				doubled(&b, p.Text)
				continue
			}
			b.WriteRune(openObject)
			b.WriteString(SourceText(f, p.Interp))
			b.WriteRune(closeObject)
		}
	}
	return b.String()
}

// doubled writes text with each literal brace doubled.
func doubled(b *strings.Builder, text string) {
	for _, c := range text {
		if c == openObject || c == closeObject {
			b.WriteRune(c)
		}
		b.WriteRune(c)
	}
}

// Unescape is a text in the form of translation files as displayed: its braces single (F6).
func Unescape(s string) string {
	return strings.NewReplacer(string([]rune{openObject, openObject}), string(openObject),
		string([]rune{closeObject, closeObject}), string(closeObject)).Replace(s)
}

// MenuRef is `menu m icon i` as declared (VIEWMODEL.md 12.4 `menu`), nil without a menu.
func MenuRef(m *syntax.ViewMenu) *vm.MenuRef {
	if m == nil || m.Menu == nil {
		return nil
	}
	out := &vm.MenuRef{Menu: m.Menu.Name}
	if m.Icon != nil {
		out.Icon = m.Icon.Name
	}
	return out
}

// SourceText is the source text of the node n of f, as written (a `when`, a `preview`).
func SourceText(f *syntax.File, n syntax.Node) string {
	sp := f.Span(n)
	return string(f.Src.Content[sp.Start:sp.End])
}
