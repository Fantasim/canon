package i18n

import (
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// letEntries adds a public top-level let's label and help (I18N.md K "v", "v.help"), and, when
// it has a define-table view (v may be local, K "v.title"), that view's own texts.
func (b *builder) letEntries(o check.Object) {
	if !isLocalDecl(o.Decl()) {
		b.letLabel(o)
	}
	view, ok := b.views[o]
	if !ok || !definesTable(o.Type()) {
		return
	}
	b.viewTopEntries(o.Name(), view)
}

// letLabel adds the let's label (its `@menu(label:)`, else the humanized name) and help (its
// doc comment).
func (b *builder) letLabel(o check.Object) {
	d, ok := o.Decl().(*syntax.LetDecl)
	if !ok {
		return
	}
	name := o.Name()
	if s := menuLabel(d); s != nil {
		b.addText(name, "", o.File(), s, Plain)
	} else {
		b.addPlain(name, "", Humanize(name))
	}
	doc := ""
	if d.Doc != nil {
		doc = d.Doc.Text
	}
	b.addPlain(join(name, syntax.PropHelp), "", doc)
}

// menuLabel is `@menu(…, label: "…")`'s text, nil without one.
func menuLabel(d *syntax.LetDecl) syntax.StrLit {
	for _, a := range d.Annotations {
		if a.Name == nil || a.Name.Name != syntax.AnnMenu {
			continue
		}
		if s := menuLabelArgValue(a); s != nil {
			return s
		}
	}
	return nil
}

// menuLabelArgValue is a's `label:` argument text, nil without one.
func menuLabelArgValue(a *syntax.Annotation) syntax.StrLit {
	for _, arg := range a.Args {
		if arg.Name == nil || arg.Name.Name != syntax.ArgLabel {
			continue
		}
		if s, ok := arg.Value.(syntax.StrLit); ok {
			return s
		}
	}
	return nil
}

// definesTable reports the type of a `load.defines` table (VIEWMODEL.md §3.2).
func definesTable(t types.Type) bool {
	tt, ok := t.Base().(*types.TableType)
	return ok && tt.Elem == types.DefineType
}

// packageCheck adds check.<name> for a named one-line check or warn at package level (I18N.md
// K1, CHK-03).
func (b *builder) packageCheck(o check.Object) {
	d, ok := o.Decl().(*syntax.CheckDecl)
	if !ok || d.Name == nil || d.Body != nil || d.Message == nil {
		return
	}
	b.addText(join(syntax.WordCheck, d.Name.Name), "", o.File(), d.Message, Template)
}
