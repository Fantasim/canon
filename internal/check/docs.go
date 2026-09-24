package check

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
)

// checkDocs is W1002 (GRAMMAR.md §9.1).
func (c *checker) checkDocs(p *pkgState) {
	for _, o := range p.all {
		if o.local {
			continue
		}
		env := c.declEnv(o)
		switch d := o.decl.(type) {
		case *syntax.RecordDecl:
			c.undocumented(env, d.Doc, d.Name, diag.KindRecord)
			c.fieldDocs(env, d.Body.Items)
		case *syntax.EnumDecl:
			c.undocumented(env, d.Doc, d.Name, diag.KindEnum)
		case *syntax.VariantDecl:
			c.undocumented(env, d.Doc, d.Name, diag.KindVariant)
			c.caseDocs(env, d)
		case *syntax.TypeDecl:
			c.undocumented(env, d.Doc, d.Name, diag.KindTypeAlias)
		case *syntax.FnDecl:
			c.fnDoc(env, d)
		}
	}
}

// fnDoc is W1002 for an `export fn`, top-level or a method.
func (c *checker) fnDoc(env *env, d *syntax.FnDecl) {
	if d.Mods != nil && d.Mods.Export.Valid() {
		c.undocumented(env, d.Doc, d.Name, diag.KindFunction)
	}
}

// fieldDocs is W1002 for the fields and export methods of a public record or case body.
func (c *checker) fieldDocs(env *env, items []syntax.RecordItem) {
	for _, it := range items {
		switch it := it.(type) {
		case *syntax.FieldDecl:
			c.undocumented(env, it.Doc, it.Name, diag.KindField)
		case *syntax.FnDecl:
			c.fnDoc(env, it)
		}
	}
}

// caseDocs is W1002 for the fields of the cases of a public variant.
func (c *checker) caseDocs(env *env, d *syntax.VariantDecl) {
	for _, it := range d.Items {
		switch it := it.(type) {
		case *syntax.VariantCase:
			if it.Body != nil {
				c.fieldDocs(env, it.Body.Items)
			}
		case *syntax.FnDecl:
			c.fnDoc(env, it)
		}
	}
}

func (c *checker) undocumented(env *env, doc *syntax.DocComment, name *syntax.Ident, kind diag.Kind) {
	if doc == nil && name != nil {
		c.warn(env, diag.W1002.At(env.span(name), kind, name.Name))
	}
}
