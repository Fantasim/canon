package i18n

import (
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// typeEntries adds every key of t (I18N.md §3.3).
func (b *builder) typeEntries(t types.Type) {
	file := b.files[t]
	switch x := t.(type) {
	case *types.RecordType:
		body := bodyInfo{fields: x.Fields, methods: x.Methods, checks: x.Checks, view: b.byT[x], file: file, docs: methodDocs(recordBodyItems(x.Decl))}
		b.recordType(x.Name, x.Doc, body)
	case *types.VariantType:
		b.variantType(x, file)
	case *types.EnumType:
		b.enumType(x.Name, x.Doc, x.Members, b.byT[x])
	}
}

// bodyInfo is a record or case body's declaration content the catalogue reads (I18N.md §3.3).
type bodyInfo struct {
	fields  []*types.Field
	methods []*types.Method
	checks  []*syntax.CheckDecl
	view    viewEntry
	file    *syntax.File
	docs    map[string]string
}

// recordType adds prefix.help, then body's keys (K2, K8).
func (b *builder) recordType(prefix, doc string, body bodyInfo) {
	b.addPlain(join(prefix, syntax.PropHelp), "", doc)
	b.typeBody(prefix, body)
}

// typeBody adds every field's keys, every named check, and, with a view, the view's own texts
// and its methods (K2): shared by a record and a variant's case.
func (b *builder) typeBody(prefix string, body bodyInfo) {
	var sc *scanned
	if body.view.decl != nil {
		sc = walkItems(body.view.decl)
	}
	for _, f := range body.fields {
		b.fieldEntries(prefix, f, sc, body.view.file)
	}
	b.checkEntries(prefix, body.checks, body.file)
	if body.view.decl == nil {
		return
	}
	b.viewTopEntries(prefix, body.view)
	b.groupsAndShows(prefix, body.view, sc)
	b.methodEntries(prefix, body.methods, sc, body.view.file, body.docs)
}

// fieldEntries adds a field's label, help, deprecation and view-only props (I18N.md K1); a case
// field with no view item of its own falls back to the label its parent record's view gives it
// through inlining (K7).
func (b *builder) fieldEntries(prefix string, f *types.Field, sc *scanned, file *syntax.File) {
	key, alt := namedForm(prefix, f.Name, syntax.WordField)
	vf, vfFile := scannedField(sc, f.Name), file
	if vf == nil {
		if src, ok := b.inline[f]; ok {
			vf, vfFile = src.vf, src.file
		}
	}
	b.memberLabel(labelInfo{key: key, alt: alt, def: Humanize(f.Name), doc: f.Doc, vf: vf, file: vfFile})
	if f.Deprecated != nil {
		b.addPlain(join(key, syntax.AnnDeprecated), "", f.Deprecated.Why)
	}
	if vf == nil {
		return
	}
	if s := fieldProp(vf, syntax.PropPlaceholder); s != nil {
		b.addText(join(key, syntax.PropPlaceholder), "", vfFile, s, Plain)
	}
	if s := fieldProp(vf, syntax.PropNone); s != nil {
		b.addText(join(key, syntax.PropNone), "", vfFile, s, Plain)
	}
	if s := fieldProp(vf, syntax.PropStep); s != nil {
		b.addText(join(key, syntax.PropStep), "", vfFile, s, Template)
	}
}

func scannedField(sc *scanned, name string) *syntax.ViewField {
	if sc == nil {
		return nil
	}
	return sc.fields[name]
}

// checkEntries adds prefix.check.<name> for every named one-line check or warn (I18N.md K8).
func (b *builder) checkEntries(prefix string, checks []*syntax.CheckDecl, file *syntax.File) {
	for _, d := range checks {
		if d.Name == nil || d.Body != nil || d.Message == nil {
			continue
		}
		key := join(prefix, syntax.WordCheck, d.Name.Name)
		b.addText(key, "", file, d.Message, Template)
	}
}

// methodEntries adds a method's label and help, only for a method a view names (I18N.md K2):
// help is the view's help prop, else the method's own doc comment (K "T.m.help").
func (b *builder) methodEntries(prefix string, methods []*types.Method, sc *scanned, file *syntax.File, docs map[string]string) {
	if sc == nil {
		return
	}
	for _, m := range methods {
		vf := sc.fields[m.Name]
		if vf == nil {
			continue
		}
		key, alt := namedForm(prefix, m.Name, syntax.WordMethod)
		b.memberLabel(labelInfo{key: key, alt: alt, def: Humanize(m.Name), doc: docs[m.Name], vf: vf, file: file})
	}
}

// recordBodyItems is d's body items, nil for a nil declaration (a built-in or a broken type).
func recordBodyItems(d *syntax.RecordDecl) []syntax.RecordItem {
	if d == nil {
		return nil
	}
	return bodyItemsOf(d.Body)
}

// bodyItemsOf is a record or case body's items, nil without one.
func bodyItemsOf(b *syntax.RecordBody) []syntax.RecordItem {
	if b == nil {
		return nil
	}
	return b.Items
}

// methodDocs maps every method declared among items to its own doc comment (I18N.md K "T.m.help").
func methodDocs(items []syntax.RecordItem) map[string]string {
	out := map[string]string{}
	for _, it := range items {
		if fn, ok := it.(*syntax.FnDecl); ok && fn.Name != nil && fn.Doc != nil {
			out[fn.Name.Name] = fn.Doc.Text
		}
	}
	return out
}
