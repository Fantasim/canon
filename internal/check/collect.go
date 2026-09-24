package check

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// collect declares every top-level name of p's files in its namespace (TYPES.md §1 step 1, §3.2).
func (c *checker) collect(p *pkgState) {
	for _, f := range p.files {
		switch f.FileKind {
		case syntax.FileSource:
			for _, d := range f.Decls {
				c.collectDecl(p, f, d)
			}
		case syntax.FileLayer:
			c.collectLayer(p, f)
		default:
		}
	}
	c.collectKeys(p)
}

// collectDecl makes the object of one top-level declaration; a BadDecl declares nothing.
func (c *checker) collectDecl(p *pkgState, f *syntax.File, d syntax.Decl) {
	kind, name, ok := declKind(d)
	if !ok {
		if e, isEntry := d.(*syntax.EntryDecl); isEntry {
			o := c.newObject(ObjEntry, entryKeyText(e.Key), p, e, f)
			c.addDecl(p, o)
			p.entries = append(p.entries, entryAt{decl: e, file: f, obj: o})
		}
		return
	}
	o := c.newObject(kind, identName(name), p, d, f)
	o.local = isLocal(d)
	if name != nil {
		c.info.Defs[name] = o
	}
	c.addDecl(p, o)
	if !namespaced(d) || name == nil {
		return
	}
	if first, dup := p.names[name.Name]; dup {
		diag.E2106.At(f.Span(name), name.Name, declSpan(first)).Report(p.bag)
		c.breakObj(o)
		return
	}
	p.names[name.Name] = o
	c.makeNamedType(p, o)
}

// addDecl lists o among p's declarations (EVALUATION.md §2.1) and the breakable objects.
func (c *checker) addDecl(p *pkgState, o *object) {
	p.pkg.Decls = append(p.pkg.Decls, o)
	p.all = append(p.all, o)
}

// declKind is the object kind of a declaration and the identifier it declares, if any.
func declKind(d syntax.Decl) (ObjKind, *syntax.Ident, bool) {
	switch d := d.(type) {
	case *syntax.ConstDecl:
		return ObjConst, d.Name, true
	case *syntax.LetDecl:
		return ObjLet, d.Name, true
	case *syntax.TypeDecl:
		return ObjTypeName, d.Name, true
	case *syntax.RecordDecl:
		return ObjTypeName, d.Name, true
	case *syntax.EnumDecl:
		return ObjTypeName, d.Name, true
	case *syntax.VariantDecl:
		return ObjTypeName, d.Name, true
	case *syntax.FnDecl:
		return ObjFn, d.Name, true
	case *syntax.WidgetDecl:
		return ObjWidget, d.Name, true
	case *syntax.CheckDecl:
		return ObjCheck, d.Name, true
	case *syntax.TestDecl:
		return ObjTest, nil, true
	}
	return 0, nil, false
}

// namespaced reports the declarations that share the package namespace (TYPES.md §3.2).
func namespaced(d syntax.Decl) bool {
	switch d.(type) {
	case *syntax.CheckDecl, *syntax.TestDecl:
		return false
	}
	return true
}

func identName(n *syntax.Ident) string {
	if n == nil {
		return ""
	}
	return n.Name
}

// isLocal reports a `local` declaration.
func isLocal(d syntax.Decl) bool {
	var m *syntax.Modifiers
	switch d := d.(type) {
	case *syntax.ConstDecl:
		m = d.Mods
	case *syntax.LetDecl:
		m = d.Mods
	case *syntax.TypeDecl:
		m = d.Mods
	case *syntax.RecordDecl:
		m = d.Mods
	case *syntax.EnumDecl:
		m = d.Mods
	case *syntax.VariantDecl:
		m = d.Mods
	case *syntax.FnDecl:
		m = d.Mods
	}
	return m != nil && m.Local.Valid()
}

// declName is the identifier a declaration binds, nil when it binds none.
func declName(n syntax.Node) *syntax.Ident {
	switch d := n.(type) {
	case syntax.Decl:
		_, name, _ := declKind(d)
		return name
	case *syntax.FieldDecl:
		return d.Name
	case *syntax.EnumMember:
		return d.Name
	case *syntax.VariantCase:
		return d.Name
	case *syntax.Param:
		return d.Name
	case *syntax.EntryItem:
		return d.Key
	}
	return nil
}

// makeNamedType creates the shell of a record, enum or variant, so every use shares its pointer;
// resolvePackage fills it.
func (c *checker) makeNamedType(p *pkgState, o *object) {
	switch d := o.decl.(type) {
	case *syntax.RecordDecl:
		o.typ = &types.RecordType{Pkg: p.path, Name: o.name, Decl: d, Doc: docText(d.Doc)}
	case *syntax.EnumDecl:
		o.typ = &types.EnumType{Pkg: p.path, Name: o.name, Ordered: d.Ordered.Valid(), Decl: d, Doc: docText(d.Doc)}
	case *syntax.VariantDecl:
		o.typ = &types.VariantType{Pkg: p.path, Name: o.name, Tag: defaultTag, Decl: d, Doc: docText(d.Doc)}
	default:
		return
	}
	c.typeObjects[o.typ] = o
}

func docText(d *syntax.DocComment) string {
	if d == nil {
		return ""
	}
	return d.Text
}

// collectLayer records a layer file's amend blocks under its layer name (EVALUATION.md §9.1).
func (c *checker) collectLayer(p *pkgState, f *syntax.File) {
	if f.Layer == nil {
		return
	}
	p.pkg.Layers[f.Layer.Name] = append(p.pkg.Layers[f.Layer.Name], f.Amends...)
}
