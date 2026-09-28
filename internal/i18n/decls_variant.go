package i18n

import (
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// variantType adds the variant's own view texts, its variant-level named checks and every case's keys (I18N.md §3.3).
func (b *builder) variantType(v *types.VariantType, file *syntax.File) {
	b.addPlain(join(v.Name, syntax.PropHelp), "", v.Doc)
	b.checkEntries(v.Name, check.VariantChecks(v.Decl), file)
	view := b.byT[v]
	var sc *scanned
	if view.decl != nil {
		sc = walkItems(view.decl)
		b.viewTopEntries(v.Name, view)
	}
	for _, c := range v.Cases {
		b.caseType(v.Name, c, file, sc)
	}
}

// caseType adds one case's keys (I18N.md K "T.c", "T.c.<part>", "T.c.f").
func (b *builder) caseType(variantName string, c *types.CaseType, file *syntax.File, variantSc *scanned) {
	key, alt := namedForm(variantName, c.Name, syntax.ArgCase)
	b.memberLabel(labelInfo{key: key, alt: alt, def: c.Name, doc: c.Doc, vf: scannedField(variantSc, c.Name), file: file})
	body := bodyInfo{fields: c.Fields, methods: c.Methods, checks: c.Checks, view: b.byT[c], file: file, docs: methodDocs(caseBodyItems(c.Variant, c.Name))}
	b.typeBody(key, body)
}

// caseBodyItems is c's own body items, found in its variant's declaration by name; nil without
// one (a built-in or a broken type).
func caseBodyItems(v *types.VariantType, name string) []syntax.RecordItem {
	if v.Decl == nil {
		return nil
	}
	for _, it := range v.Decl.Items {
		if vc, ok := it.(*syntax.VariantCase); ok && vc.Name != nil && vc.Name.Name == name {
			return bodyItemsOf(vc.Body)
		}
	}
	return nil
}
