package render

import (
	"strings"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/i18n"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/views/encode"
	"github.com/fantasim/canonlang/internal/views/shape"
)

// translations are the non-empty translation entries of each package, by language and key
// (I18N.md 4): the first of a key given twice (E1705), files in path order.
type translations map[string]map[string]map[string]*syntax.TranslationEntry

func readTranslations(prog *check.Program) translations {
	out := translations{}
	if prog == nil {
		return out
	}
	for _, p := range prog.Packages {
		for _, f := range p.Files {
			if f.FileKind == syntax.FileTranslation && f.Lang != nil {
				out.file(p.Path, f)
			}
		}
	}
	return out
}

// file adds the entries of the translation file f of pkg; an empty text is missing (I18N.md F5).
func (t translations) file(pkg string, f *syntax.File) {
	if t[pkg] == nil {
		t[pkg] = map[string]map[string]*syntax.TranslationEntry{}
	}
	byKey := t[pkg][f.Lang.Name]
	if byKey == nil {
		byKey = map[string]*syntax.TranslationEntry{}
		t[pkg][f.Lang.Name] = byKey
	}
	for _, e := range f.Entries {
		if e.Key == nil || empty(e.Text) {
			continue
		}
		if key := syntax.Qualified(e.Key); byKey[key] == nil {
			byKey[key] = e
		}
	}
}

// Translated is the translation of the key segs of pkg in lang, false for the source language
// (lang ""), a missing one, or one check marked BrokenTranslations (I18N.md T2): it renders as
// though missing, so the caller falls back to the source template (VIEWMODEL.md X3, X6).
func (r *Renderer) Translated(pkg, lang string, segs []string) (syntax.StrLit, bool) {
	if lang == "" {
		return nil, false
	}
	e, ok := r.tr[pkg][lang][strings.Join(segs, dot)]
	if !ok || r.in.Program != nil && r.in.Program.Info.BrokenTranslations[e] {
		return nil, false
	}
	return e.Text, true
}

// empty reports a text with nothing in it (I18N.md F5).
func empty(s syntax.StrLit) bool {
	switch x := s.(type) {
	case *syntax.RawStringLit:
		return x.Value == ""
	case *syntax.StringLit:
		for _, p := range x.Parts {
			if p.Text != "" || p.Interp != nil {
				return false
			}
		}
		return true
	}
	return true
}

// plainText is the plain text keyed segs of pkg in lang: its translation, else its source text
// (I18N.md B1); false when neither exists.
func (r *Renderer) plainText(pkg, lang string, segs []string) (string, bool) {
	if s, ok := r.Translated(pkg, lang, segs); ok {
		if t, isPlain := encode.PlainText(s); isPlain {
			return t, true
		}
	}
	src, ok := r.in.Texts.Source(pkg, segs...)
	return encode.Unescape(src), ok
}

// memberLabel is an enum member's label in lang, its Canon name without one (VIEWMODEL.md X4).
func (r *Renderer) memberLabel(m *value.Member, lang string) string {
	name := m.CanonText()
	if t, ok := r.plainText(m.Enum.Pkg, lang, []string{m.Enum.Name, i18n.MemberSeg(name)}); ok {
		return t
	}
	return name
}

// noneLabel is the `none` text of the field x reads, a field of self (`{note}`) or of what a
// selector reads (`{self.note}`, `{a.b}`), in lang (VIEWMODEL.md X4, C38): catalogued, else as
// written (a text without a letter, I18N.md L7).
func (r *Renderer) noneLabel(x syntax.Expr, self value.Value, lang string) (string, bool) {
	decl, f := r.fieldRead(x, self)
	if f == nil {
		return "", false // not a field: `none` (X4)
	}
	pkg, segs := noneKey(decl, f)
	if t, ok := r.plainText(pkg, lang, segs); ok {
		return t, true
	}
	return r.in.Index.Field(f).TextIn(syntax.PropNone, pkg)
}

// NoneKey is the key of the `none` text of the field x reads, of self or of what a selector
// reads (VIEWMODEL.md X4, I18N.md 3.3 `T.f.none`); false when x reads no field.
func (r *Renderer) NoneKey(x syntax.Expr, self value.Value) (string, []string, bool) {
	decl, f := r.fieldRead(x, self)
	if f == nil {
		return "", nil, false
	}
	pkg, segs := noneKey(decl, f)
	return pkg, segs, true
}

// noneKey is the key of the `none` text of the field f of decl (I18N.md 3.3 `T.f.none`).
func noneKey(decl types.Type, f *types.Field) (string, []string) {
	pkg, segs := i18n.FieldKey(decl, f)
	return pkg, append(segs, syntax.PropNone)
}

// fieldRead is the field x reads and the record or case type declaring it; nil for none.
func (r *Renderer) fieldRead(x syntax.Expr, self value.Value) (types.Type, *types.Field) {
	decl, name := r.readField(x, self)
	if decl == nil {
		return nil, nil
	}
	for _, f := range encode.FieldsOf(decl) {
		if f.Name == name {
			return decl.Base(), f
		}
	}
	return nil, nil
}

// readField is the record or case type and the field name x reads, nil when x reads no field.
func (r *Renderer) readField(x syntax.Expr, self value.Value) (types.Type, string) {
	if r.in.Program == nil {
		return nil, ""
	}
	info := r.in.Program.Info
	switch e := shape.Unparen(x).(type) {
	case *syntax.IdentExpr:
		rec, isRecord := self.(*value.Record)
		if o := info.Uses[e]; isRecord && o != nil && o.Kind() == check.ObjField {
			return rec.T, e.Name
		}
	case *syntax.SelectorExpr:
		if sel := info.Selections[e]; sel != nil && sel.Obj != nil && sel.Obj.Kind() == check.ObjField && e.Name != nil {
			return shape.StripOptional(sel.Recv), e.Name.Name
		}
	}
	return nil, ""
}
