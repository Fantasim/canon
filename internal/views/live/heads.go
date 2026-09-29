package live

import (
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/i18n"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/views/encode"
	"github.com/fantasim/canonlang/internal/views/render"
)

// headItem is a view item heading a value, its key's last segment and the renderer's method.
type headItem struct {
	kind   syntax.NodeKind
	word   string
	render func(*render.Renderer, *value.Record, string) (string, bool)
}

// heads are a record's title, subtitle and preview in the session's language.
type heads struct {
	title, subtitle Text
	preview         string
}

// heads renders rec's heads with its magic names m; name is its title without a view (V7).
func (s *session) heads(rec *value.Record, m render.Magic, name string) heads {
	r := s.place(rec, m)
	defer s.unplace()
	h := heads{title: s.headText(r, rec, titleItem, name, true), subtitle: s.headText(r, rec, subtitleItem, "", true)}
	h.preview, _ = r.Preview(rec)
	return h
}

// sourceTitle is rec's view title in the source language, placed with m, as S9 compares titles;
// "" without one. The expressions a translation shares with the source are evaluated once (memo).
func (s *session) sourceTitle(rec *value.Record, m render.Magic) string {
	if _, titled := s.shown.Head(rec, titleItem.kind); !titled {
		return ""
	}
	defer s.unplace()
	v, _ := s.place(rec, m).Title(rec, "")
	return v
}

// headText is rec's title or subtitle rendered by r (VIEWMODEL.md S2, X6, API.md V8): name when
// no view gives rec one (V7), OK false when it fails (V11); refs false inside a ref's target title.
func (s *session) headText(r *render.Renderer, rec *value.Record, it headItem, name string, refs bool) Text {
	d, ok := s.shown.Head(rec, it.kind)
	if !ok {
		return Text{Value: name, OK: true}
	}
	v, ok := it.render(r, rec, s.lang)
	if !ok {
		return Text{}
	}
	return Text{Value: v, OK: true, Fallback: s.fallbackIn(d.View.Pkg, keyOf(d, it), templateOf(d, it), rec, refs)}
}

// place is the renderer of rec's expressions with its magic names m (VIEWMODEL.md 3.4), which
// the memo is read with until unplace.
func (s *session) place(rec *value.Record, m render.Magic) *render.Renderer {
	s.at = placed{self: rec, magic: m}
	return s.shown.At(rec, m)
}

// unplace ends place.
func (s *session) unplace() { s.at = placed{} }

// asTarget reads the memo as a ref's target e renders, in its own place m (render's Target;
// log-2026-09-29 M4 U9b-r), until the returned function restores the place before.
func (s *session) asTarget(e *value.Record, m render.Magic) func() {
	prev := s.at
	s.at = placed{self: e, magic: m}
	return func() { s.at = prev }
}

// keyOf is the key of the item it of the view d (I18N.md 3.3 `T.title`, `T.subtitle`).
func keyOf(d render.Described, it headItem) []string {
	return append(slices.Clip(d.Prefix), it.word)
}

// templateOf is the source template of the item it of the view d, nil for none.
func templateOf(d render.Described, it headItem) syntax.StrLit {
	switch x := d.View.Item(it.kind).(type) {
	case *syntax.ViewTitle:
		return x.Text
	case *syntax.ViewSubtitle:
		return x.Text
	}
	return nil
}

// fellBack reports a text of pkg keyed segs shown in the source language because the session's
// language lacks its translation (I18N.md B1), every one in a language the project does not
// have (log-2026-09-29 M4 U9); false for a text without a key.
func (s *session) fellBack(pkg string, segs []string) bool {
	r := s.in.I18N[pkg]
	if s.lang == "" || r == nil || r.Catalogue == nil {
		return false
	}
	lang := r.Languages[s.lang]
	if lang == nil {
		lang = &i18n.Language{}
	}
	return i18n.Fallback(r.Catalogue, lang, strings.Join(segs, dot)).Fallback
}

// label is the plain text of pkg keyed segs in the session's language, falling back to the
// source (I18N.md B1); src when the catalogue has no such key (a text without a letter, L6).
func (s *session) label(pkg string, segs []string, src string) string {
	r := s.in.I18N[pkg]
	key := strings.Join(segs, dot)
	if r == nil || r.Catalogue == nil {
		return src
	}
	if _, ok := r.Catalogue.Lookup(key); !ok {
		return src
	}
	return encode.Unescape(i18n.Fallback(r.Catalogue, r.Languages[s.lang], key).Value)
}
