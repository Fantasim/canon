package control

import (
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/views/encode"
	"github.com/fantasim/canonlang/internal/views/shape"
)

// Props are an item's view properties by name: the value each was given (VIEWMODEL.md 3.5).
type Props map[string]prop

// prop is a property's value, the package of the view that gives it (K7) and its file.
type prop struct {
	value syntax.Expr
	pkg   string
	file  *syntax.File
}

// merge adds the properties of b, given by the view v, that p does not hold yet; p is created
// when needed.
func merge(p Props, b *syntax.BraceLit, v View) Props {
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
			p[fi.Name.Name] = prop{value: fi.Value, pkg: v.Pkg, file: v.File}
		}
	}
	return p
}

// name is a property given as a name (`unit: hp`, `control: slider`) or as a string standing
// for one (`unit: "hp"`, TYPES.md 10.3); "" otherwise.
func (p Props) name(key string) string {
	switch v := shape.Unparen(p[key].value).(type) {
	case *syntax.IdentExpr:
		return v.Name
	case *syntax.StringLit, *syntax.RawStringLit:
		s, _ := text(v)
		return s
	}
	return ""
}

// ident is a property given as a name (`control: slider`, `icon: gem`); "" otherwise: a string
// names no control, icon or tone (E1609, E1610).
func (p Props) ident(key string) string {
	if id, ok := shape.Unparen(p[key].value).(*syntax.IdentExpr); ok {
		return id.Name
	}
	return ""
}

// Member is an `icon` or `tone` property: a member of the studio's enum, bare (`gem`) or
// qualified (`Icon.gem`, VIEWMODEL.md 3.5), as its bare name; "" otherwise.
func (p Props) Member(key string) string {
	if s, ok := shape.Unparen(p[key].value).(*syntax.SelectorExpr); ok && s.Name != nil {
		return s.Name.Name
	}
	return p.ident(key)
}

// TextIn is a property given as a string without interpolation by a view of pkg: only the
// owner's texts are keys of its catalogue (I18N.md K7, VIEWMODEL.md J9).
func (p Props) TextIn(key, pkg string) (string, bool) {
	if p[key].pkg != pkg {
		return "", false
	}
	return text(p[key].value)
}

// text is a string without interpolation, unescaped, parentheses removed.
func text(e syntax.Expr) (string, bool) { return encode.PlainText(shape.Unparen(e)) }

// TemplateIn is a template property given by a view of pkg, in the source form of translation
// files (a `step`, I18N.md F6); false otherwise.
func (p Props) TemplateIn(key, pkg string) (string, bool) {
	s, ok := shape.Unparen(p[key].value).(syntax.StrLit)
	if !ok || p[key].pkg != pkg || p[key].file == nil {
		return "", false
	}
	return encode.TemplateSource(p[key].file, s), true
}

// Source is the source text of a property's value (VIEWMODEL.md 12.4 `when`); false when not given.
func (p Props) Source(key string) (string, bool) {
	pr, ok := p[key]
	if !ok || pr.file == nil {
		return "", false
	}
	return encode.SourceText(pr.file, pr.value), true
}

// True reports a property given the literal `true` (`hidden`, `readonly`).
func (p Props) True(key string) bool {
	b, ok := shape.Unparen(p[key].value).(*syntax.BoolLit)
	return ok && b.Value
}
