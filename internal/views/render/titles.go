package render

import (
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/i18n"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/views/control"
	"github.com/fantasim/canonlang/internal/views/shape"
)

// Described is a view describing a value and its key prefix (I18N.md 3.3).
type Described struct {
	View   control.View
	Prefix []string
}

// Head is the first view describing e that holds an item of kind k: its case's view first, then
// its variant's or record's, or its define table's (VIEWMODEL.md 3.2, G7); false for none.
func (r *Renderer) Head(e *value.Record, k syntax.NodeKind) (Described, bool) {
	for _, d := range r.views(e) {
		if d.View.Item(k) != nil {
			return d, true
		}
	}
	return Described{}, false
}

// views are the views describing e, most specific first.
func (r *Renderer) views(e *value.Record) []Described {
	if e.T.Base().Kind() == types.Define {
		return r.defineView(e)
	}
	var out []Described
	for _, t := range Viewed(e.T) {
		if v, ok := r.in.Index.ViewOf(t); ok {
			_, prefix := i18n.TypeKey(t)
			out = append(out, Described{View: v, Prefix: prefix})
		}
	}
	return out
}

// Viewed are the targets whose views describe a value of type t, most specific first: a case
// then its variant, a record (an applied one's declaration).
func Viewed(t types.Type) []types.Type {
	switch x := t.Base().(type) {
	case *types.CaseType:
		return []types.Type{x, x.Variant}
	case *types.AppliedRecord:
		return []types.Type{x.Rec}
	}
	return []types.Type{t.Base()}
}

// defineView is the view of the define table holding e (VIEWMODEL.md G7).
func (r *Renderer) defineView(e *value.Record) []Described {
	if e.Ident == nil || e.Ident.Coll == nil || r.in.Program == nil {
		return nil
	}
	c, p := e.Ident.Coll, shape.Package(r.in.Program, e.Ident.Coll.Pkg)
	if p == nil {
		return nil
	}
	for _, o := range p.Decls {
		if o.Name() != c.Name || o.Kind() != check.ObjLet {
			continue
		}
		if v, ok := r.in.Index.LetView(o); ok {
			return []Described{{View: v, Prefix: []string{o.Name()}}}
		}
	}
	return nil
}

// Title is e's rendered view title in lang (VIEWMODEL.md S2, S8); false when its type has no
// view title or it fails to render (X7).
func (r *Renderer) Title(e *value.Record, lang string) (string, bool) {
	return r.headText(e, syntax.KindViewTitle, syntax.WordTitle, lang)
}

// Subtitle is e's rendered view subtitle in lang (S2); false as for Title.
func (r *Renderer) Subtitle(e *value.Record, lang string) (string, bool) {
	return r.headText(e, syntax.KindViewSubtitle, syntax.WordSubtitle, lang)
}

// headText renders e's title or subtitle: the translated template in lang when there is one,
// else the source template (X6).
func (r *Renderer) headText(e *value.Record, k syntax.NodeKind, word, lang string) (string, bool) {
	d, ok := r.Head(e, k)
	if !ok {
		return "", false
	}
	t := Template{Pkg: d.View.Pkg, Key: append(slices.Clip(d.Prefix), word), Source: headTemplate(d.View.Item(k))}
	return r.Template(t, e, lang)
}

// headTemplate is the template of a title or subtitle item.
func headTemplate(it syntax.ViewItem) syntax.StrLit {
	if t, ok := it.(*syntax.ViewTitle); ok {
		return t.Text
	}
	return it.(*syntax.ViewSubtitle).Text
}

// ID is the magic name `id` of a table or define-table entry: its key (VIEWMODEL.md 3.4); nil
// for another value.
func ID(e *value.Record) value.Value {
	if e == nil || e.Ident == nil || e.Ident.Coll == nil || e.Ident.Coll.KeyedBy != nil {
		return nil
	}
	return keyValue(e.Ident.Key)
}

// keyValue is an entry's key as a value: a String, or an Int for an integer key.
func keyValue(k value.Key) value.Value {
	if k.IsInt {
		return &value.Int{V: k.I, T: types.IntType}
	}
	return &value.Str{V: k.S, T: types.StringType}
}

// refText is a ref as a view renders it (S8): its target's title when it has one, in the target's
// own place (log-2026-09-29 M4 U9b-r), else its key; inside that title a ref renders its key (one
// level), and a title that fails renders the key.
func (r *Renderer) refText(ref *value.Ref, lang string) string {
	rt, ok := ref.T.Base().(*types.RefType)
	if !ok || r.inTarget {
		return ref.Key.Text()
	}
	if e, at := r.Target(rt.Target, ref.Key); e != nil {
		r.inTarget = true
		t, ok := r.At(e, at).Title(e, lang)
		r.inTarget = false
		if ok {
			return t
		}
	}
	return ref.Key.Text()
}

// Target is the entry of coll keyed k in this build and the magic names of its own position
// (Position); nil for none (a field of an enclosing record).
func (r *Renderer) Target(coll *types.Collection, k value.Key) (*value.Record, Magic) {
	es := r.in.Colls.Entries(coll)
	byKey, ok := r.keys[coll]
	if !ok {
		byKey = map[value.Key]int{}
		for i, e := range es {
			if e.Ident != nil {
				byKey[e.Ident.Key] = i
			}
		}
		r.keys[coll] = byKey
	}
	i, ok := byKey[k]
	if !ok {
		return nil, Magic{}
	}
	return es[i], Position(coll, i)
}

// Position are the magic names of the entry i of coll besides its `id` (VIEWMODEL.md 3.4): a
// keyed list's element has its 1-based `index`.
func Position(coll *types.Collection, i int) Magic {
	if coll.KeyedBy == nil {
		return Magic{}
	}
	return Magic{Index: &value.Int{V: int64(i + 1), T: types.IntType}}
}

// Disambiguated is a title two entries of one collection share, shown with the entry's key's
// canonical text (VIEWMODEL.md S9): `<title> (<key>)`.
func Disambiguated(title, key string) string { return title + keyOpen + key + keyClose }
