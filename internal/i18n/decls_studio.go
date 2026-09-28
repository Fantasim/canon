package i18n

import (
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// findDecl is the studio package's top-level declaration named name of kind, nil without one.
func (b *builder) findDecl(name string, kind check.ObjKind) check.Object {
	for _, o := range b.pkg.Decls {
		if o.Kind() == kind && o.Name() == name {
			return o
		}
	}
	return nil
}

// studioMenu adds Menu.<member>[.help] for the studio package's Menu enum (I18N.md K3): a
// member's label comes from the enum's own view, else its Canon name; the enum's own doc
// contributes nothing (K3: "its other declarations contribute nothing").
func (b *builder) studioMenu() {
	o := b.findDecl(syntax.StudioMenu, check.ObjTypeName)
	if o == nil {
		return
	}
	e, ok := o.Type().(*types.EnumType)
	if !ok {
		return
	}
	view := b.byT[e]
	var sc *scanned
	if view.decl != nil {
		sc = walkItems(view.decl)
	}
	for _, m := range e.Members {
		key, alt := namedForm(syntax.StudioMenu, m.Name, syntax.WordMember)
		b.memberLabel(labelInfo{key: key, alt: alt, def: m.Name, doc: m.Doc, vf: scannedField(sc, m.Name), file: view.file})
	}
}

// studioUnits adds units.<unit>.suffix for every entry of the studio package's `units` table
// whose suffix is translatable (I18N.md K3, U1).
func (b *builder) studioUnits() {
	o := b.findDecl(syntax.StudioUnits, check.ObjLet)
	if o == nil {
		return
	}
	d, ok := o.Decl().(*syntax.LetDecl)
	if !ok {
		return
	}
	lit, ok := d.Value.(*syntax.BraceLit)
	if !ok {
		return
	}
	file := o.File()
	for _, item := range lit.Items {
		e, ok := item.(*syntax.EntryItem)
		if !ok || e.Key == nil {
			continue
		}
		if s, sFile := unitSuffix(b.info, e.Value, file); s != nil {
			key := join(syntax.StudioUnits, e.Key.Name, studioSuffixProp)
			b.addText(key, "", sFile, s, Plain)
		}
	}
}

// unitSuffix is v's `suffix` field item and its declaring file (never file when the value
// follows a `const` declared elsewhere), nil, nil without one written.
func unitSuffix(info *check.Info, v *syntax.BraceLit, file *syntax.File) (syntax.StrLit, *syntax.File) {
	if v == nil {
		return nil, nil
	}
	for _, it := range v.Items {
		fi, ok := it.(*syntax.FieldItem)
		if !ok || fi.Name == nil || fi.Name.Name != studioSuffixProp {
			continue
		}
		return suffixText(info, fi.Value, file, map[check.Object]bool{})
	}
	return nil, nil
}

// suffixText is e as a string literal and the file it is written in, following a `const`
// reference through check.Info (Uses, Decl, File); seen stops a cycle between consts.
func suffixText(info *check.Info, e syntax.Expr, file *syntax.File, seen map[check.Object]bool) (syntax.StrLit, *syntax.File) {
	switch x := e.(type) {
	case syntax.StrLit:
		return x, file
	case *syntax.ParenExpr:
		return suffixText(info, x.X, file, seen)
	case *syntax.IdentExpr:
		o := info.Uses[x]
		if o == nil || o.Kind() != check.ObjConst || seen[o] {
			return nil, nil
		}
		seen[o] = true
		cd, ok := o.Decl().(*syntax.ConstDecl)
		if !ok {
			return nil, nil
		}
		return suffixText(info, cd.Value, o.File(), seen)
	default:
		return nil, nil
	}
}
