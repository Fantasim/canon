package live

import (
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/i18n"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/views/control"
	"github.com/fantasim/canonlang/internal/views/encode"
	"github.com/fantasim/canonlang/internal/views/render"
	"github.com/fantasim/canonlang/internal/views/shape"
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
	defer s.place(rec, m)()
	h := heads{title: s.headText(s.shown, rec, titleItem, name), subtitle: s.headText(s.shown, rec, subtitleItem, "")}
	h.preview, _ = s.shown.Preview(rec)
	return h
}

// sourceTitle is rec's view title in the source language, placed with m, as S9 compares titles;
// "" without one. The expressions a translation shares with the source are evaluated once (memo).
func (s *session) sourceTitle(rec *value.Record, m render.Magic) string {
	if _, titled := s.headOf(rec, titleItem); !titled {
		return ""
	}
	defer s.place(rec, m)()
	v, _ := s.shown.Title(rec, "")
	return v
}

// headText is rec's title or subtitle rendered by r (VIEWMODEL.md S2, X6, API.md V8): name when
// no view gives rec one (V7), OK false when it fails (V11).
func (s *session) headText(r *render.Renderer, rec *value.Record, it headItem, name string) Text {
	d, ok := s.headOf(rec, it)
	if !ok {
		return Text{Value: name, OK: true}
	}
	v, ok := it.render(r, rec, s.lang)
	if !ok {
		return Text{}
	}
	return Text{Value: v, OK: true, Fallback: s.fallbackIn(d.view.Pkg, keyOf(d, it), templateOf(d, it), rec, r != s.target)}
}

// place renders rec's expressions with its magic names m until the returned function is called.
func (s *session) place(rec *value.Record, m render.Magic) func() {
	s.at.self, s.at.magic = rec, m
	return func() { s.at.self, s.at.magic = nil, render.Magic{} }
}

// keyOf is the key of the item it of the view d (I18N.md 3.3 `T.title`, `T.subtitle`).
func keyOf(d described, it headItem) []string { return append(slices.Clip(d.prefix), it.word) }

// templateOf is the source template of the item it of the view d, nil for none.
func templateOf(d described, it headItem) syntax.StrLit {
	switch x := d.view.Item(it.kind).(type) {
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

// described is a view describing a value and its key prefix (I18N.md 3.3).
type described struct {
	view   control.View
	prefix []string
}

// headOf is the first view describing rec that holds the item it (VIEWMODEL.md 3.2, G7): its
// case's, then its variant's or record's, or its define table's.
func (s *session) headOf(rec *value.Record, it headItem) (described, bool) {
	for _, d := range s.describing(rec) {
		if d.view.Item(it.kind) != nil {
			return d, true
		}
	}
	return described{}, false
}

// describing are the views describing rec, most specific first.
func (s *session) describing(rec *value.Record) []described {
	if rec.T.Base().Kind() == types.Define {
		return s.defineView(rec)
	}
	var out []described
	for _, t := range viewed(rec.T) {
		if v, ok := s.index.ViewOf(t); ok {
			_, prefix := i18n.TypeKey(t)
			out = append(out, described{view: v, prefix: prefix})
		}
	}
	return out
}

// viewed are the targets whose views describe a value of type t: a case then its variant, a
// record (an applied one's declaration).
func viewed(t types.Type) []types.Type {
	switch x := t.Base().(type) {
	case *types.CaseType:
		return []types.Type{x, x.Variant}
	case *types.AppliedRecord:
		return []types.Type{x.Rec}
	}
	return []types.Type{t.Base()}
}

// defineView is the view of the define table holding rec (VIEWMODEL.md G7).
func (s *session) defineView(rec *value.Record) []described {
	if rec.Ident == nil || rec.Ident.Coll == nil {
		return nil
	}
	c := rec.Ident.Coll
	p := shape.Package(s.in.Program, c.Pkg)
	if p == nil {
		return nil
	}
	i := slices.IndexFunc(p.Decls, func(o check.Object) bool { return o.Name() == c.Name && o.Kind() == check.ObjLet })
	if i < 0 {
		return nil
	}
	if v, ok := s.index.LetView(p.Decls[i]); ok {
		return []described{{view: v, prefix: []string{c.Name}}}
	}
	return nil
}
