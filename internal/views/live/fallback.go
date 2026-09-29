package live

import (
	"strings"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/i18n"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/views/encode"
	"github.com/fantasim/canonlang/internal/views/render"
	"github.com/fantasim/canonlang/internal/views/shape"
)

// fallbackIn reports a template keyed segs of pkg, rendered for self, shown with a fallback
// (API.md V8): its own, or an enum label, `none` text or ref's target title it interpolates
// (refs only outside a target title, S8); read from what rendering evaluated (log-2026-09-29 M4 U9).
func (s *session) fallbackIn(pkg string, segs []string, src syntax.StrLit, self *value.Record, refs bool) bool {
	if s.lang == "" {
		return false
	}
	tpl := s.translated(pkg, segs)
	if tpl == nil && s.catalogued(pkg, segs) {
		return true
	}
	if tpl == nil {
		tpl = src
	}
	sl, ok := tpl.(*syntax.StringLit)
	if !ok {
		return false
	}
	for _, p := range sl.Parts {
		if p.Interp != nil && labelled(s.in.Program.Info.Types[p.Interp.X]) && s.valueFellBack(p.Interp.X, self, refs) {
			return true
		}
	}
	return false
}

// valueFellBack reports an interpolation of self whose enum label, `none` text, or ref's target
// title when refs, is shown with a fallback (VIEWMODEL.md X4).
func (s *session) valueFellBack(x syntax.Expr, self *value.Record, refs bool) bool {
	v, ok := s.memo.read(x, self, s.magicOf(self))
	if !ok {
		return false
	}
	switch y := v.(type) {
	case *value.Member:
		return s.fellBack(y.Enum.Pkg, []string{y.Enum.Name, i18n.MemberSeg(y.CanonText())})
	case *value.None:
		pkg, segs, ok := s.noneKey(x, self)
		return ok && s.fellBack(pkg, segs)
	case *value.Ref:
		if refs {
			return s.targetFellBack(y)
		}
	}
	return false
}

// targetFellBack reports a ref's target title shown with a fallback; a title that failed shows the
// key, which is none (S8).
func (s *session) targetFellBack(r *value.Ref) bool {
	rt, ok := r.T.Base().(*types.RefType)
	if !ok {
		return false
	}
	e := s.entry(rt.Target, r.Key)
	if e == nil {
		return false
	}
	d, titled := s.headOf(e, titleItem)
	if !titled || !s.rendered(d.view.Pkg, keyOf(d, titleItem), templateOf(d, titleItem), e) {
		return false
	}
	return s.fallbackIn(d.view.Pkg, keyOf(d, titleItem), templateOf(d, titleItem), e, false)
}

// rendered reports the template keyed segs of pkg that the session's language renders for self
// evaluated every interpolation (X7).
func (s *session) rendered(pkg string, segs []string, src syntax.StrLit, self *value.Record) bool {
	tpl := s.translated(pkg, segs)
	if tpl == nil {
		tpl = src
	}
	sl, ok := tpl.(*syntax.StringLit)
	if !ok {
		return tpl != nil
	}
	for _, p := range sl.Parts {
		if p.Interp == nil {
			continue
		}
		if _, ok := s.memo.read(p.Interp.X, self, s.magicOf(self)); !ok {
			return false
		}
	}
	return true
}

// noneKey is the key of the `none` text of the field x reads, of self or of what a selector reads
// (VIEWMODEL.md X4, I18N.md 3.3 `T.f.none`); false when x reads no field.
func (s *session) noneKey(x syntax.Expr, self *value.Record) (string, []string, bool) {
	info := s.in.Program.Info
	var decl types.Type
	name := ""
	switch e := shape.Unparen(x).(type) {
	case *syntax.IdentExpr:
		if o := info.Uses[e]; o != nil && o.Kind() == check.ObjField {
			decl, name = self.T, e.Name
		}
	case *syntax.SelectorExpr:
		if sel := info.Selections[e]; sel != nil && sel.Obj != nil && sel.Obj.Kind() == check.ObjField && e.Name != nil {
			decl, name = shape.StripOptional(sel.Recv), e.Name.Name
		}
	}
	for _, f := range encode.FieldsOf(decl) {
		if f.Name == name {
			pkg, segs := i18n.FieldKey(decl, f)
			return pkg, append(segs, syntax.PropNone), true
		}
	}
	return "", nil, false
}

// labelled reports a static type whose values may render by a label, a title or a `none` text
// (VIEWMODEL.md X4): an optional, a ref, an enum, or a type that may hold one; nil when unknown.
func labelled(t types.Type) bool {
	return t == nil || shape.KindIn(t, types.Optional, types.Ref, types.Enum, types.LitUnion, types.TypeApp, types.DepUnion)
}

// magicOf are the magic names rec renders with: its `id`, and its place's when it is placed.
func (s *session) magicOf(rec *value.Record) render.Magic {
	m := render.Magic{ID: idOf(rec)}
	if rec == s.at.self {
		m = withMagic(m, s.at.magic)
	}
	return m
}

// catalogued reports a key of pkg's catalogue (I18N.md 3.3): a text with a letter.
func (s *session) catalogued(pkg string, segs []string) bool {
	r := s.in.I18N[pkg]
	if r == nil || r.Catalogue == nil {
		return false
	}
	_, ok := r.Catalogue.Lookup(strings.Join(segs, dot))
	return ok
}

// translated is the template keyed segs of pkg the session's language renders instead of the
// source, as render does: the first non-empty entry of pkg's files for it in path order, nil when
// there is none or it is broken (I18N.md F3, F5, T2; VIEWMODEL.md X6).
func (s *session) translated(pkg string, segs []string) syntax.StrLit {
	p := shape.Package(s.in.Program, pkg)
	if s.lang == "" || p == nil {
		return nil
	}
	key := strings.Join(segs, dot)
	for _, f := range p.Files {
		if f.FileKind != syntax.FileTranslation || f.Lang == nil || f.Lang.Name != s.lang {
			continue
		}
		for _, e := range f.Entries {
			if e.Key == nil || empty(e.Text) || syntax.Qualified(e.Key) != key {
				continue
			}
			if s.in.Program.Info.BrokenTranslations[e] {
				return nil
			}
			return e.Text
		}
	}
	return nil
}

// empty reports a text with nothing in it: a missing translation (I18N.md F5).
func empty(t syntax.StrLit) bool {
	switch x := t.(type) {
	case *syntax.RawStringLit:
		return x.Value == ""
	case *syntax.StringLit:
		for _, p := range x.Parts {
			if p.Text != "" || p.Interp != nil {
				return false
			}
		}
	}
	return true
}
