package table

import (
	"slices"

	"github.com/fantasim/canonlang/api/vm"
	"github.com/fantasim/canonlang/internal/i18n"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/views/control"
	"github.com/fantasim/canonlang/internal/views/encode"
	"github.com/fantasim/canonlang/internal/wire"
)

// Tables completes table controls with what the views of their elements say.
type Tables struct {
	index *control.Index
	texts *encode.Texts
	res   *control.Resolver
}

// New completes tables with the views of index, texts for their "Add" noun; Bind gives the
// resolver the cells and filters are resolved with.
func New(index *control.Index, texts *encode.Texts) *Tables {
	return &Tables{index: index, texts: texts}
}

// Bind resolves the cells and filters with r, whose Env.Table is t's Complete.
func (t *Tables) Bind(r *control.Resolver) { t.res = r }

// Complete is ctl, the table control of a collection of type c, completed (VIEWMODEL.md T1): its
// key (`$id` for a table, the key field of a keyed list), rows movable (T2; a let loaded from
// files says otherwise), its columns (7.2), filters (7.3) and the element view's `singular`.
func (t *Tables) Complete(c types.Type, ctl vm.Control) vm.Control {
	elem, key := element(c)
	ctl.Orderable = true
	ctl.Columns = []vm.Column{}
	if _, isTable := c.Base().(*types.TableType); isTable {
		ctl.Key = wire.KeyID
	} else if key != nil {
		ctl.Key = key.Name
	}
	v, _ := t.index.ViewOf(viewed(elem))
	ctl.Singular = t.singular(elem, v)
	if vt, isVariant := elem.Base().(*types.VariantType); isVariant {
		ctl.Columns = withCase(t.columns(caseFields(vt), nil, key, ctl.Key != "", v))
		ctl.Filters = t.filters(caseFields(vt), v, vt)
		return ctl
	}
	keys := encode.Keys(elem)
	named := func(name string) []encode.Keyed { return encode.Named(keys, name) }
	ctl.Columns = t.columns(named, t.auto(keys, key), key, ctl.Key != "", v)
	ctl.Filters = t.filters(named, v, nil)
	return ctl
}

// namer is the keyed fields a column or filter name places (VIEWMODEL.md 3.3, L18, T6a).
type namer func(string) []encode.Keyed

// caseFields places a name among the case fields of the variant v, in case order (T6a, 3.3 rule
// 3): each keyed `<case>.<field>`, nested inline case fields below it.
func caseFields(v *types.VariantType) namer {
	return func(name string) []encode.Keyed {
		var out []encode.Keyed
		for _, c := range v.Cases {
			for _, k := range encode.Named(encode.Keys(c), name) {
				k.Key = c.Name + dot + k.Key
				out = append(out, k)
			}
		}
		return out
	}
}

// withCase puts the case column after the entry column of a table of variants (T6a).
func withCase(cols []vm.Column) []vm.Column {
	at := 0
	if len(cols) > 0 && cols[0].Field == entryColumn {
		at = 1
	}
	return slices.Insert(cols, at, vm.Column{Field: encode.KeyCase, Mode: modeCase})
}

// viewed is the type a view of the element targets: its record, variant or case, aliases,
// refinements and arguments removed.
func viewed(elem types.Type) types.Type {
	if a, ok := elem.Base().(*types.AppliedRecord); ok {
		return a.Rec
	}
	return elem.Base()
}

// element is a collection's element type, and its key field for a keyed list.
func element(c types.Type) (types.Type, *types.Field) {
	switch x := c.Base().(type) {
	case *types.ListType:
		return x.Elem, x.KeyedBy
	case *types.TableType:
		return x.Elem, nil
	}
	return c, nil
}

// Singular is the `singular` of the view of the record or variant elem (T3, 12.5), for a list or
// cards control; absent without one.
func (t *Tables) Singular(elem types.Type) vm.TextRef {
	v, _ := t.index.ViewOf(viewed(elem))
	return t.singular(elem, v)
}

// singular is the element view's `singular` (T3); absent without one.
func (t *Tables) singular(elem types.Type, v control.View) vm.TextRef {
	it, ok := v.Item(syntax.KindViewSingular).(*syntax.ViewSingular)
	if !ok {
		return vm.TextRef{}
	}
	text, _ := encode.PlainText(it.Text)
	pkg, segs := i18n.TypeKey(elem)
	return t.texts.Text(pkg, text, append(segs, syntax.WordSingular)...)
}
