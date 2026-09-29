package live

import (
	"strings"

	"github.com/fantasim/canonlang/internal/i18n"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
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
	tpl, translated := s.shown.Translated(pkg, s.lang, segs)
	if !translated && s.catalogued(pkg, segs) {
		return true
	}
	if !translated {
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
	return ok && s.fellBackValue(v, x, self, refs)
}

// fellBackValue reports v, x's value for self (nil x: a method's), rendered with a fallback: an
// enum label, the `none` text of the field x reads, or a ref's target title when refs (X4).
func (s *session) fellBackValue(v value.Value, x syntax.Expr, self *value.Record, refs bool) bool {
	switch y := v.(type) {
	case *value.Member:
		return s.fellBack(y.Enum.Pkg, []string{y.Enum.Name, i18n.MemberSeg(y.CanonText())})
	case *value.None:
		pkg, segs, ok := s.shown.NoneKey(x, self)
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
	e, at := s.shown.Target(rt.Target, r.Key)
	if e == nil {
		return false
	}
	defer s.asTarget(e, at)()
	d, titled := s.shown.Head(e, titleItem.kind)
	if !titled || !s.rendered(d.View.Pkg, keyOf(d, titleItem), templateOf(d, titleItem), e) {
		return false
	}
	return s.fallbackIn(d.View.Pkg, keyOf(d, titleItem), templateOf(d, titleItem), e, false)
}

// rendered reports the template keyed segs of pkg that the session's language renders for self
// evaluated every interpolation (X7).
func (s *session) rendered(pkg string, segs []string, src syntax.StrLit, self *value.Record) bool {
	tpl, translated := s.shown.Translated(pkg, s.lang, segs)
	if !translated {
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

// labelled reports a static type whose values may render by a label, a title or a `none` text
// (VIEWMODEL.md X4): an optional, a ref, an enum, or a type that may hold one; nil when unknown.
func labelled(t types.Type) bool {
	return t == nil || shape.KindIn(t, types.Optional, types.Ref, types.Enum, types.LitUnion, types.TypeApp, types.DepUnion)
}

// magicOf are the magic names rec renders with, as render gives them: its `id`, and its place's
// when it is the value placed (At).
func (s *session) magicOf(rec *value.Record) render.Magic {
	m := render.Magic{ID: render.ID(rec)}
	if rec == s.at.self {
		m = m.With(s.at.magic)
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
