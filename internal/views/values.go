package views

import (
	"github.com/fantasim/canonlang/api/vm"
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/i18n"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/views/control"
	"github.com/fantasim/canonlang/internal/views/encode"
	"github.com/fantasim/canonlang/internal/views/shape"
)

// values is the `values` section (VIEWMODEL.md 12.6): every public top-level let, numbered by
// its position among them; a broken one is left out (J4).
func (b *builder) values() {
	order := 0
	for _, o := range b.pkg.Decls {
		if b.ctx.Err() != nil {
			return // Build reports the cancellation
		}
		d, ok := o.Decl().(*syntax.LetDecl)
		if !ok || o.Kind() != check.ObjLet || shape.Local(d) {
			continue
		}
		if !b.broken(o) {
			b.m.Values[valueID(b.pkg.Path, o.Name())] = b.value(o, d, order)
		}
		order++
	}
}

// value is one public let: its type, label and help, menu (N1), control, editability at its
// root (API.md 7), `@reload`, the layers amending it, whether it failed (J4) and its sources.
func (b *builder) value(o check.Object, d *syntax.LetDecl, order int) vm.Value {
	name, pkg := o.Name(), b.pkg.Path
	label, ok := menuLabel(d)
	if !ok {
		label = i18n.Humanize(name)
	}
	doc := ""
	if d.Doc != nil {
		doc = d.Doc.Text
	}
	v := vm.Value{
		Name: name, Order: order, Type: b.defs.Expr(nil, o.Type()),
		Label: b.texts.Label(pkg, label, name, name), Help: b.texts.Text(pkg, doc, name, syntax.PropHelp),
		Menu: b.valueMenu(o, d), Control: b.valueControl(o, d),
		Reload: shape.Annotation(d, syntax.AnnReload) != nil, Layers: b.layersOf(o), Sources: b.sources(o, d),
	}
	v.Editable, v.Reason = editable(b.in.Program.Info, d.Value)
	if b.in.Force != nil {
		_, settled := b.colls.Let(pkg, name)
		v.Failed = !settled
	}
	return v
}

// valueID is the id of a value or search index (J8): `"<package>:<let>"`.
func valueID(pkg, name string) string { return pkg + colon + name }

// broken reports a declaration left out of the view model (TYPES.md 1, J4).
func (b *builder) broken(o check.Object) bool {
	return b.in.Program.Info.Broken[o] || o.Type() == nil || o.Type().Kind() == types.Error
}

// editable is the edit mode at a let's root and, for `none`, its reason (12.6, API.md W1).
func editable(info *check.Info, e syntax.Expr) (mode, reason string) {
	switch shape.SourceForm(info, e) {
	case shape.FormLiteral:
		return editCanon, ""
	case shape.FormJSON:
		return editJSON, ""
	case shape.FormFormat:
		return editNone, reasonFormat
	}
	return editNone, reasonComputed
}

// valueControl is the control of the value (4); a collection indexed for search names its index
// (T16), and rows loaded from files do not move (T2).
func (b *builder) valueControl(o check.Object, d *syntax.LetDecl) vm.Control {
	ctl := b.res.Value(nil, o.Type())
	if ctl.Kind != control.CtlTable {
		return ctl
	}
	if b.indexed(o) {
		ctl.Search = valueID(b.pkg.Path, o.Name())
	}
	if loadsDir(d) || b.entriesOf(o) > 0 {
		ctl.Orderable = false
	}
	return ctl
}

// valueMenu is a value's menu (VIEWMODEL.md N1): its `@menu`, else the `menu` of the view, in the
// value's package, of the record it holds or holds a list, keyed list or table of.
func (b *builder) valueMenu(o check.Object, d *syntax.LetDecl) *vm.MenuRef {
	if a := shape.Annotation(d, syntax.AnnMenu); a != nil {
		return annotationMenu(a)
	}
	t := shape.MenuType(o.Type())
	if t == nil {
		return nil
	}
	v, ok := b.index.ViewOf(t)
	if !ok || v.Pkg != b.pkg.Path {
		return nil
	}
	m, _ := v.Item(syntax.KindViewMenu).(*syntax.ViewMenu)
	return encode.MenuRef(m)
}

// annotationMenu is `@menu(m, icon: i)` (G23).
func annotationMenu(a *syntax.Annotation) *vm.MenuRef {
	out := &vm.MenuRef{}
	for _, arg := range a.Args {
		q, ok := arg.Value.(*syntax.QualifiedName)
		switch {
		case !ok || len(q.Parts) != 1:
		case arg.Name == nil:
			out.Menu = q.Parts[0].Name
		case arg.Name.Name == syntax.PropIcon:
			out.Icon = q.Parts[0].Name
		}
	}
	if out.Menu == "" {
		return nil
	}
	return out
}

// menuLabel is `@menu(label: "…")` (G23, N3).
func menuLabel(d *syntax.LetDecl) (string, bool) {
	a := shape.Annotation(d, syntax.AnnMenu)
	if a == nil {
		return "", false
	}
	for _, arg := range a.Args {
		if arg.Name != nil && arg.Name.Name == syntax.ArgLabel {
			if s, ok := arg.Value.(syntax.StrLit); ok {
				return encode.PlainText(s)
			}
		}
	}
	return "", false
}
