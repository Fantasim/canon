package layout

import (
	"github.com/fantasim/canonlang/api/vm"
	"github.com/fantasim/canonlang/internal/i18n"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/views/encode"
	"github.com/fantasim/canonlang/internal/views/shape"
)

// heads are the view-level items of the view (VIEWMODEL.md 3.6, 12.4): `title`, `subtitle`,
// `preview`, `singular`, `plural` and `menu`.
func (l *lay) heads(out *vm.View) {
	if l.view.Decl == nil {
		return
	}
	for _, it := range l.view.Decl.Items {
		switch x := it.(type) {
		case *syntax.ViewTitle:
			out.Title = l.template(x.Text, syntax.WordTitle)
		case *syntax.ViewSubtitle:
			out.Subtitle = l.template(x.Text, syntax.WordSubtitle)
		case *syntax.ViewSingular:
			out.Singular = l.plain(x.Text, syntax.WordSingular)
		case *syntax.ViewPlural:
			out.Plural = l.plain(x.Text, syntax.WordPlural)
		case *syntax.ViewPreview:
			out.Preview = l.preview(x)
		case *syntax.ViewMenu:
			out.Menu = encode.MenuRef(x)
		}
	}
}

// template is a template item: its source, and its text reference when it is a key (J9).
func (l *lay) template(s syntax.StrLit, segs ...string) *vm.Template {
	if s == nil {
		return nil
	}
	return &vm.Template{Template: i18n.SourceText(l.view.File, s), Text: l.in.Texts.Key(l.pkg, l.key(segs...)...)}
}

// plain is a plain text item's reference (J9).
func (l *lay) plain(s syntax.StrLit, segs ...string) vm.TextRef {
	if s == nil {
		return vm.TextRef{}
	}
	text, _ := encode.PlainText(s)
	return l.in.Texts.Text(l.pkg, text, l.key(segs...)...)
}

// preview is the preview expression's source and its asset root and extensions; none when its
// type is not an asset (E1612).
func (l *lay) preview(p *syntax.ViewPreview) *vm.Preview {
	t := l.in.Info.Types[p.X]
	if t == nil {
		return nil
	}
	a := shape.LayersOf(shape.StripOptional(t)).Asset
	if a == nil {
		return nil
	}
	return &vm.Preview{Expr: encode.SourceText(l.view.File, p.X), Root: l.in.Assets.Root(a), Ext: a.Exts}
}
