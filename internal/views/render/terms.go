package render

import (
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
)

// Terms are e's rendered `search` terms (VIEWMODEL.md S2): strings as they are, integers in
// decimal, enum members and cases by Canon name, refs by key, assets by file name, lists
// flattened, `none` and a failed term omitted.
func (r *Renderer) Terms(e *value.Record) []string {
	d, ok := r.Head(e, syntax.KindViewSearch)
	if !ok {
		return nil
	}
	var out []string
	for _, x := range d.View.Item(syntax.KindViewSearch).(*syntax.ViewSearch).Items {
		if v, ok := r.Value(x, e, r.magicOf(e)); ok {
			out = appendTerm(out, v)
		}
	}
	return out
}

// appendTerm adds the text of v, or of each of its elements.
func appendTerm(out []string, v value.Value) []string {
	switch x := v.(type) {
	case *value.List:
		for _, el := range x.Elems {
			out = appendTerm(out, el)
		}
		return out
	case *value.None:
		return out
	case *value.Ref:
		return append(out, x.Key.Text())
	case *value.Str:
		return append(out, x.V)
	}
	return append(out, v.CanonText())
}

// Preview is the asset file name e's view `preview` gives (S2); false for none.
func (r *Renderer) Preview(e *value.Record) (string, bool) {
	d, ok := r.Head(e, syntax.KindViewPreview)
	if !ok {
		return "", false
	}
	v, ok := r.Value(d.View.Item(syntax.KindViewPreview).(*syntax.ViewPreview).X, e, r.magicOf(e))
	if s, isStr := v.(*value.Str); ok && isStr && s.V != "" {
		return s.V, true
	}
	return "", false
}
