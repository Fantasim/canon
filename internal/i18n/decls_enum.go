package i18n

import (
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// enumType adds prefix.help and, for every member, prefix.<e> and prefix.<e>.help (I18N.md K
// "T.help", "T.e", "T.e.help"): a member's label comes from the enum's own view, else its Canon
// name as is (VIEWMODEL.md X1); retired members keep their keys (K9).
func (b *builder) enumType(prefix, doc string, members []*types.Member, view viewEntry) {
	b.addPlain(join(prefix, syntax.PropHelp), "", doc)
	var sc *scanned
	if view.decl != nil {
		sc = walkItems(view.decl)
	}
	for _, m := range members {
		key, alt := namedForm(prefix, m.Name, syntax.WordMember)
		b.memberLabel(labelInfo{key: key, alt: alt, def: m.Name, doc: m.Doc, vf: scannedField(sc, m.Name), file: view.file})
	}
}
