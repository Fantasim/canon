package render

import (
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/views/control"
	"github.com/fantasim/canonlang/internal/views/encode"
	"github.com/fantasim/canonlang/internal/views/shape"
)

// head is a view item that describes an entry, with its view's package and key prefix.
type head struct {
	item   syntax.ViewItem
	pkg    string
	prefix []string
}

// described is a view describing an entry and its key prefix (I18N.md 3.3).
type described struct {
	view   control.View
	prefix []string
}

// Head is the item of kind k of the views describing e: its case's view first, then its
// variant's or record's, or its define table's (VIEWMODEL.md 3.2, G7); false for none.
func (r *Renderer) head(e *value.Record, k syntax.NodeKind) (head, bool) {
	for _, d := range r.views(e) {
		if it := d.view.Item(k); it != nil {
			return head{item: it, pkg: d.view.Pkg, prefix: d.prefix}, true
		}
	}
	return head{}, false
}

// views are the views describing e, most specific first.
func (r *Renderer) views(e *value.Record) []described {
	var out []described
	add := func(t types.Type) {
		if v, ok := r.in.Index.ViewOf(t); ok {
			_, prefix := encode.TypeKey(t)
			out = append(out, described{view: v, prefix: prefix})
		}
	}
	switch t := e.T.Base().(type) {
	case *types.CaseType:
		add(t)
		add(t.Variant)
	case *types.AppliedRecord:
		add(t.Rec)
	default:
		if t.Kind() == types.Define {
			return r.defineView(e)
		}
		add(t)
	}
	return out
}

// defineView is the view of the define table holding e (VIEWMODEL.md G7).
func (r *Renderer) defineView(e *value.Record) []described {
	if e.Ident == nil || r.in.Program == nil {
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
			return []described{{view: v, prefix: []string{o.Name()}}}
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
	h, ok := r.head(e, k)
	if !ok {
		return "", false
	}
	tpl := headTemplate(h.item)
	if s, found := r.translated(h.pkg, lang, append(slices.Clip(h.prefix), word)); found {
		tpl = s
	}
	return r.template(tpl, e, Magic{ID: id(e)}, lang)
}

// headTemplate is the template of a title or subtitle item.
func headTemplate(it syntax.ViewItem) syntax.StrLit {
	if t, ok := it.(*syntax.ViewTitle); ok {
		return t.Text
	}
	return it.(*syntax.ViewSubtitle).Text
}

// id is the magic name `id` of a table or define-table entry: its key (VIEWMODEL.md 3.4); nil
// for another value.
func id(e *value.Record) value.Value {
	if e.Ident == nil || e.Ident.Coll == nil || e.Ident.Coll.KeyedBy != nil {
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

// refText is a ref as a view renders it (S8): its target's title when it has one, else its key;
// inside that title a ref renders its key (one level), and a title that fails renders the key.
func (r *Renderer) refText(ref *value.Ref, lang string) string {
	rt, ok := ref.T.Base().(*types.RefType)
	if !ok || r.inTarget {
		return ref.Key.Text()
	}
	if e := r.entry(rt.Target, ref.Key); e != nil {
		r.inTarget = true
		t, ok := r.Title(e, lang)
		r.inTarget = false
		if ok {
			return t
		}
	}
	return ref.Key.Text()
}

// entry is the entry of coll keyed k in this build, nil for none.
func (r *Renderer) entry(coll *types.Collection, k value.Key) *value.Record {
	if r.keys == nil {
		r.keys = map[*types.Collection]map[value.Key]*value.Record{}
	}
	byKey, ok := r.keys[coll]
	if !ok {
		byKey = map[value.Key]*value.Record{}
		for _, e := range r.in.Colls.Entries(coll) {
			if e.Ident != nil {
				byKey[e.Ident.Key] = e
			}
		}
		r.keys[coll] = byKey
	}
	return byKey[k]
}

// Disambiguated is a title two entries of one collection share, shown with the entry's key's
// canonical text (VIEWMODEL.md S9): `<title> (<key>)`.
func Disambiguated(title, key string) string { return title + keyOpen + key + keyClose }
