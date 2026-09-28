package control

import (
	"strings"

	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/views/shape"
)

// Props are an item's view properties by name: the value each was given (VIEWMODEL.md §3.5).
type Props map[string]prop

// prop is a property's value and the package of the view that gives it (K7).
type prop struct {
	value syntax.Expr
	pkg   string
}

// merge adds the properties of b, given by a view of pkg, that p does not hold yet; p is
// created when needed.
func merge(p Props, b *syntax.BraceLit, pkg string) Props {
	if b == nil {
		return p
	}
	for _, it := range b.Items {
		fi, ok := it.(*syntax.FieldItem)
		if !ok || fi.Name == nil || fi.Value == nil {
			continue
		}
		if p == nil {
			p = Props{}
		}
		if _, given := p[fi.Name.Name]; !given {
			p[fi.Name.Name] = prop{value: fi.Value, pkg: pkg}
		}
	}
	return p
}

// Name is a property given as a name (`unit: hp`, `control: slider`) or as a string standing
// for one (`unit: "hp"`, TYPES.md 10.3); "" otherwise.
func (p Props) Name(key string) string {
	switch v := shape.Unparen(p[key].value).(type) {
	case *syntax.IdentExpr:
		return v.Name
	case *syntax.StringLit, *syntax.RawStringLit:
		s, _ := text(v)
		return s
	}
	return ""
}

// TextIn is a property given as a string without interpolation by a view of pkg: only the
// owner's texts are keys of its catalogue (I18N.md K7, VIEWMODEL.md J9).
func (p Props) TextIn(key, pkg string) (string, bool) {
	if p[key].pkg != pkg {
		return "", false
	}
	return text(p[key].value)
}

// text is a string without interpolation, unescaped.
func text(e syntax.Expr) (string, bool) {
	switch v := shape.Unparen(e).(type) {
	case *syntax.StringLit:
		var b strings.Builder
		for _, part := range v.Parts {
			if part.Interp != nil {
				return "", false
			}
			b.WriteString(part.Text)
		}
		return b.String(), true
	case *syntax.RawStringLit:
		return v.Value, true
	}
	return "", false
}

// isTrue reports a property given the literal `true`.
func (p Props) isTrue(key string) bool {
	b, ok := shape.Unparen(p[key].value).(*syntax.BoolLit)
	return ok && b.Value
}
