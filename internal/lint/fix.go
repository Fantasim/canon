package lint

import (
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/types"
)

// fixable reports that renaming field of h to name is a fix canon edit accepts (API.md L10, E29, E32).
func (p *pass) fixable(h holder, field *types.Field, name string) bool {
	if !project.IsIdent(name) || field.Stable || p.text.Has(h.typ) || !p.safe(h.pkg) {
		return false
	}
	for _, f := range h.fields {
		if f != field && f.Name == name {
			return false
		}
	}
	for _, m := range h.methods {
		if m.Name == name {
			return false
		}
	}
	return true
}

// renamePath is the E27 path of the field called name in h: `<pkg>:<Rec>.<field>`, a case's with its variant.
func renamePath(h holder, name string) string {
	return h.pkg + packageMark + h.path + pathDot + name
}
