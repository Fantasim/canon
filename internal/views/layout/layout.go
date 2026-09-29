package layout

import (
	"slices"
	"strconv"

	"github.com/fantasim/canonlang/api/vm"
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/i18n"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/views/control"
	"github.com/fantasim/canonlang/internal/views/encode"
)

// Input is what the views of a package are resolved with.
type Input struct {
	Info   *check.Info
	Index  *control.Index
	Res    *control.Resolver
	Texts  *encode.Texts
	Assets *encode.Assets
}

// Section is the `views` of a package (VIEWMODEL.md 12.4): one resolved view per record and
// variant among described, by qualified name.
func Section(in Input, described []types.Type) map[string]vm.View {
	out := map[string]vm.View{}
	for _, t := range described {
		switch x := t.(type) {
		case *types.RecordType:
			if !in.Index.Broken(x) {
				out[x.String()] = in.body(x, viewRecord)
			}
		case *types.VariantType:
			if !in.Index.Broken(x) { // a broken `view V.c` removes only itself (J4, D5)
				out[x.String()] = in.variant(x)
			}
		}
	}
	return out
}

// variant is a variant's view: its heads, its case selector (C9) and a case view per case (D5).
func (in Input) variant(v *types.VariantType) vm.View {
	l := in.lay(v)
	out := vm.View{Kind: viewVariant, Declared: l.view.Decl != nil, Cases: map[string]vm.View{}}
	l.heads(&out)
	if sel, _, ok := in.Res.Choice(nil, v); ok {
		out.Selector = &sel
	}
	for _, c := range v.Cases {
		out.Cases[c.Name] = in.body(c, viewCase)
	}
	return out
}

// body is the view of a record or a case (12.4 record view): its heads, sections, field views,
// methods and `show` lines.
func (in Input) body(t types.Type, kind string) vm.View {
	l := in.lay(t)
	out := vm.View{Kind: kind, Declared: l.view.Decl != nil}
	l.heads(&out)
	out.Fields = l.fields()
	out.Sections = l.sections()
	out.Methods = l.methods()
	out.Shows = l.shows()
	return out
}

// lay is the layout of t being resolved: its view (none: the default layout, D5), key prefix
// and field keys.
type lay struct {
	in      Input
	t       types.Type
	view    control.View
	pkg     string
	prefix  []string
	keys    []encode.Keyed
	showIDs map[*syntax.ViewShow]string // each show line's id, `_<n>` for an unnamed one
}

func (in Input) lay(t types.Type) *lay {
	viewed := t
	if a, ok := t.(*types.AppliedRecord); ok {
		viewed = a.Rec
	}
	v, _ := in.Index.ViewOf(viewed)
	pkg, prefix := i18n.TypeKey(t)
	l := &lay{in: in, t: t, view: v, pkg: pkg, prefix: prefix, keys: encode.Keys(t), showIDs: map[*syntax.ViewShow]string{}}
	unnamed := 0
	l.each(func(it syntax.Node, _ *syntax.ViewGroup) {
		s, ok := it.(*syntax.ViewShow)
		switch {
		case !ok:
		case s.ID != nil:
			l.showIDs[s] = s.ID.Name
		default:
			l.showIDs[s] = unnamedShow + strconv.Itoa(unnamed)
			unnamed++
		}
	})
	return l
}

// each calls fn on every item of the view and every member of its groups, in view order, with
// the group holding it (nil at view level).
func (l *lay) each(fn func(syntax.Node, *syntax.ViewGroup)) {
	if l.view.Decl == nil {
		return
	}
	for _, it := range l.view.Decl.Items {
		fn(it, nil)
		if g, ok := it.(*syntax.ViewGroup); ok {
			for _, m := range g.Members {
				fn(m, g)
			}
		}
	}
}

// key is the key segs below the view's prefix (I18N.md 3.3).
func (l *lay) key(segs ...string) []string { return slices.Concat(l.prefix, segs) }

// named is what a member item names, nil for nothing (E1602 is the view checks').
func (l *lay) named(f *syntax.ViewField) check.Object {
	if f.Name == nil {
		return nil
	}
	return l.in.Info.NameUses[f.Name]
}
