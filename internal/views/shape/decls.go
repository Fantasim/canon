package shape

import (
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
)

// Local reports a declaration written `local`.
func Local(d syntax.Node) bool {
	var m *syntax.Modifiers
	switch x := d.(type) {
	case *syntax.RecordDecl:
		m = x.Mods
	case *syntax.VariantDecl:
		m = x.Mods
	case *syntax.EnumDecl:
		m = x.Mods
	case *syntax.TypeDecl:
		m = x.Mods
	case *syntax.LetDecl:
		m = x.Mods
	}
	return m != nil && m.Local.Valid()
}

// Annotation is the let's annotation name, nil for none.
func Annotation(d *syntax.LetDecl, name string) *syntax.Annotation {
	for _, a := range d.Annotations {
		if a.Name != nil && a.Name.Name == name {
			return a
		}
	}
	return nil
}

// Package is the checked package of the path, nil when the program does not hold it.
func Package(prog *check.Program, path string) *check.Package {
	if prog == nil {
		return nil
	}
	for _, p := range prog.Packages {
		if p.Path == path {
			return p
		}
	}
	return nil
}
