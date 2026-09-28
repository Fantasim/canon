package rules

import (
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/views/shape"
)

// studio is the package project.studio names: the first declaration of each name, as its
// namespace keeps it (VIEWMODEL.md G16, RES-06).
type studio struct {
	path   string
	decls  map[string]check.Object
	broken map[check.Object]bool
}

// studioOf is the loaded studio package, nil when project.studio names none that is loaded.
func studioOf(prog *check.Program, bags check.Bags, path string) *studio {
	for _, p := range prog.Packages {
		if p.Path != path || path == "" || bags[p.Path] == nil {
			continue
		}
		s := &studio{path: path, decls: map[string]check.Object{}, broken: prog.Info.Broken}
		for _, o := range p.Decls {
			if _, dup := s.decls[o.Name()]; !dup && o.Name() != "" {
				s.decls[o.Name()] = o
			}
		}
		return s
	}
	return nil
}

// known reports whether the studio's enum enumName has a member name; decided is false when
// that cannot be told: no studio loaded, or a broken enum.
func (s *studio) known(enumName, name string) (known, decided bool) {
	if s == nil {
		return false, false
	}
	o := s.decls[enumName]
	if o == nil || o.Kind() != check.ObjTypeName {
		return false, true
	}
	if s.broken[o] {
		return false, false
	}
	e, ok := shape.Unalias(o.Type()).(*types.EnumType)
	if !ok {
		return false, true
	}
	for _, m := range e.Members {
		if m.Name == name {
			return true, true
		}
	}
	return false, true
}

// studioName reports E1610 when the studio's enum enumName has no member id (VIEWMODEL.md G16).
func (c *checker) studioName(bag *diag.Bag, at source.Span, enumName string, kind diag.Kind, id string) {
	if known, decided := c.studio.known(enumName, id); decided && !known {
		diag.E1610.At(at, kind, id, c.studio.path).Report(bag)
	}
}

// studioProp is `icon` or `tone`: a member of the studio's enum enumName.
func studioProp(enumName string, kind diag.Kind) func(*view, named, *syntax.FieldItem) {
	return func(v *view, _ named, fi *syntax.FieldItem) {
		if id, ok := fi.Value.(*syntax.IdentExpr); ok && v.c.info.Uses[id] == nil {
			v.c.studioName(v.bag, v.span(id), enumName, kind, id.Name)
		}
	}
}

// menu is `menu m icon i`: a name check did not resolve is the studio's to judge (G16).
func (v *view) menu(it syntax.ViewItem) {
	m := it.(*syntax.ViewMenu)
	if m.Menu != nil && v.c.info.NameUses[m.Menu] == nil {
		v.c.studioName(v.bag, v.span(m.Menu), syntax.StudioMenu, diag.KindMenu, m.Menu.Name)
	}
	if m.Icon != nil && v.c.info.NameUses[m.Icon] == nil {
		v.c.studioName(v.bag, v.span(m.Icon), syntax.StudioIcon, diag.KindIcon, m.Icon.Name)
	}
}

// menuAnnotations checks each `@menu` of p: on a local let it is E1632, and its menu and icon
// are the studio's (VIEWMODEL.md G23, G16).
func (c *checker) menuAnnotations(p *check.Package, bag *diag.Bag) {
	for _, f := range p.Files {
		if f.FileKind != syntax.FileSource {
			continue
		}
		for _, d := range f.Decls {
			if l, ok := d.(*syntax.LetDecl); ok {
				c.menuAnnotation(f, l, bag)
			}
		}
	}
}

// menuAnnotation is E1632 on a let that is not public; `@menu` elsewhere is syntax's E1118
// (log-2026-09-28 views V1 review calls).
func (c *checker) menuAnnotation(f *syntax.File, l *syntax.LetDecl, bag *diag.Bag) {
	for _, a := range l.Annotations {
		if a.Name == nil || a.Name.Name != syntax.AnnMenu {
			continue
		}
		if l.Mods != nil && l.Mods.Local.Valid() {
			diag.E1632.At(f.Span(a)).Report(bag)
		}
		for _, arg := range a.Args {
			c.menuArg(f, arg, bag)
		}
	}
}

// menuArg is `@menu`'s menu, or its `icon:`, a member of the studio's enum.
func (c *checker) menuArg(f *syntax.File, arg *syntax.AnnotationArg, bag *diag.Bag) {
	q, ok := arg.Value.(*syntax.QualifiedName)
	if !ok || len(q.Parts) != 1 {
		return
	}
	switch {
	case arg.Name == nil:
		c.studioName(bag, f.Span(q), syntax.StudioMenu, diag.KindMenu, q.Parts[0].Name)
	case arg.Name.Name == syntax.PropIcon:
		c.studioName(bag, f.Span(q), syntax.StudioIcon, diag.KindIcon, q.Parts[0].Name)
	}
}

// navigation reports W1640 for each editable public value of p without a menu (VIEWMODEL.md N4).
func (c *checker) navigation(p *check.Package, bag *diag.Bag) {
	menus := c.menuTypes(p)
	for _, o := range p.Decls {
		d, ok := o.Decl().(*syntax.LetDecl)
		if !ok || o.Kind() != check.ObjLet || c.info.Broken[o] || isError(o.Type()) || d.Mods != nil && d.Mods.Local.Valid() {
			continue
		}
		if f := shape.SourceForm(c.info, d.Value); f != shape.FormLiteral && f != shape.FormJSON || hasMenu(d) || menus[menuType(o.Type())] {
			continue
		}
		diag.W1640.At(o.File().Span(d.Name), o.Name()).Report(bag)
	}
}

// menuTypes are the records whose view in p has a `menu` (VIEWMODEL.md N1).
func (c *checker) menuTypes(p *check.Package) map[types.Type]bool {
	out := map[types.Type]bool{}
	for _, f := range p.Files {
		fileViews(f, func(d *syntax.ViewDecl) {
			o := c.info.NameUses[d.Type]
			if o == nil || d.Case != nil || o.Kind() != check.ObjTypeName {
				return
			}
			for _, it := range d.Items {
				if it.Kind() == syntax.KindViewMenu {
					out[shape.Unalias(o.Type())] = true
				}
			}
		})
	}
	return out
}

// menuType is the record T a value of type t is, or a list, keyed list or table of; nil for none.
func menuType(t types.Type) types.Type {
	b := t.Base()
	if e := shape.ElemOf(b); e != nil {
		b = e.Base()
	} else if tt, ok := b.(*types.TableType); ok {
		b = tt.Elem.Base()
	}
	if a, ok := b.(*types.AppliedRecord); ok {
		return a.Rec
	}
	if _, ok := b.(*types.RecordType); ok {
		return b
	}
	return nil
}

// hasMenu reports a let with `@menu` (VIEWMODEL.md G23).
func hasMenu(d *syntax.LetDecl) bool {
	for _, a := range d.Annotations {
		if a.Name != nil && a.Name.Name == syntax.AnnMenu {
			return true
		}
	}
	return false
}
