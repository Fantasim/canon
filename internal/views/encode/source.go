package encode

import (
	"strings"

	"github.com/fantasim/canonlang/api/vm"
	"github.com/fantasim/canonlang/internal/syntax"
)

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
