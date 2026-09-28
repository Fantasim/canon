package layout

import (
	"slices"

	"github.com/fantasim/canonlang/api/vm"
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/i18n"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/views/control"
	"github.com/fantasim/canonlang/internal/views/encode"
)

// fields are the field views of every field key (VIEWMODEL.md 12.4 `fields`, L17).
func (l *lay) fields() map[string]vm.FieldView {
	out := make(map[string]vm.FieldView, len(l.keys))
	for _, k := range l.keys {
		out[k.Key] = l.field(k)
	}
	return out
}

// field is one field view: its label (X1) and help, its control (4), read-only reason and only
// value (C34, C43), `hidden`, `when` and its texts, its case path and relative path (L18) and
// its environment variable. A case field's texts are its case's keys (I18N.md K7).
func (l *lay) field(k encode.Keyed) vm.FieldView {
	f, p := k.Field, l.in.Index.Field(k.Field)
	pkg, segs := encode.FieldKey(k.Decl, f)
	label, ok := l.in.Index.FieldLabel(f)
	if !ok {
		label = i18n.Humanize(f.Name)
	}
	help, ok := p.TextIn(syntax.PropHelp, pkg)
	if !ok {
		help = f.Doc
	}
	out := vm.FieldView{
		Label:   l.in.Texts.Label(pkg, label, f.Name, segs...),
		Help:    l.in.Texts.Text(pkg, help, append(slices.Clip(segs), syntax.PropHelp)...),
		Control: l.in.Res.Field(k.Decl, f),
		Hidden:  p.True(syntax.PropHidden),
		When:    source(p, syntax.PropWhen),
		Case:    k.Case,
		Path:    k.Path,
	}
	out.Readonly, out.Single = l.in.Res.ReadOnly(f)
	out.Placeholder = l.prop(p, pkg, segs, syntax.PropPlaceholder)
	out.None = l.prop(p, pkg, segs, syntax.PropNone)
	if step, ok := p.TemplateIn(syntax.PropStep, pkg); ok {
		out.Step = l.in.Texts.Text(pkg, step, append(slices.Clip(segs), syntax.PropStep)...)
	}
	if f.Input != nil {
		out.Env = f.Input.Env
	}
	return out
}

// prop is a text property's reference, keyed below segs (I18N.md 3.3); absent when not given
// by a view of pkg.
func (l *lay) prop(p control.Props, pkg string, segs []string, name string) vm.TextRef {
	text, ok := p.TextIn(name, pkg)
	if !ok {
		return vm.TextRef{}
	}
	return l.in.Texts.Text(pkg, text, append(slices.Clip(segs), name)...)
}

// source is a property's source text, nil when not given.
func source(p control.Props, name string) *string {
	if s, ok := p.Source(name); ok {
		return &s
	}
	return nil
}

// methods are the methods the view names (3.3, L23): their label (the view label, else the
// humanized name), help (`help`, else the doc comment), `hidden` and `when`.
func (l *lay) methods() map[string]vm.MethodView {
	var out map[string]vm.MethodView
	l.each(func(n syntax.Node, _ *syntax.ViewGroup) {
		f, ok := n.(*syntax.ViewField)
		o := l.namedObj(f, ok)
		if o == nil || o.Kind() != check.ObjMethod {
			return
		}
		if out == nil {
			out = map[string]vm.MethodView{}
		}
		out[o.Name()] = l.method(o)
	})
	return out
}

// namedObj is what the member item f names, nil when n is no member item.
func (l *lay) namedObj(f *syntax.ViewField, ok bool) check.Object {
	if !ok {
		return nil
	}
	return l.named(f)
}

func (l *lay) method(o check.Object) vm.MethodView {
	name := o.Name()
	p := l.in.Index.Item(l.t, name)
	label, ok := l.in.Index.ItemLabel(l.t, name)
	if !ok {
		label = i18n.Humanize(name)
	}
	help, ok := p.TextIn(syntax.PropHelp, l.pkg)
	if fn, isFn := o.Decl().(*syntax.FnDecl); !ok && isFn && fn.Doc != nil {
		help = fn.Doc.Text
	}
	segs := l.key(encode.MethodSeg(name))
	return vm.MethodView{
		Label:  l.in.Texts.Label(l.pkg, label, name, segs...),
		Help:   l.in.Texts.Text(l.pkg, help, append(segs, syntax.PropHelp)...),
		Hidden: p.True(syntax.PropHidden),
		When:   source(p, syntax.PropWhen),
	}
}

// shows are the view's `show` lines by id (5.8, 12.4): their label and template.
func (l *lay) shows() map[string]vm.ShowView {
	var out map[string]vm.ShowView
	l.each(func(n syntax.Node, _ *syntax.ViewGroup) {
		s, ok := n.(*syntax.ViewShow)
		if !ok {
			return
		}
		if out == nil {
			out = map[string]vm.ShowView{}
		}
		id := l.showIDs[s]
		label, _ := encode.PlainText(s.Label)
		segs := l.key(syntax.WordShow, id)
		out[id] = vm.ShowView{
			Label: l.in.Texts.Label(l.pkg, label, label, segs...),
			Text:  vm.Template{Template: encode.TemplateSource(l.view.File, s.Template), Text: l.in.Texts.Key(l.pkg, append(segs, syntax.WordText)...)},
		}
	})
	return out
}
