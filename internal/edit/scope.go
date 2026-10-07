package edit

import (
	"slices"

	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/syntax"
)

// Scope is the packages that may declare each op's path root or RenameName target, by the parsed
// declarations, every public candidate of an unqualified root; and whether a Rename or RenameName
// asks for their importers too (API.md E17a, P6, P7, E27, E11, E33). In name order, each once.
func Scope(units []*project.Unit, ops []Operation) (pkgs []string, importers bool) {
	for _, op := range ops {
		switch op.Kind {
		case OpRenameName:
			pkgs, importers = append(pkgs, nameScope(units, op.Path)...), true
		default:
			pkgs = append(pkgs, pathScope(units, op.Path)...)
			importers = importers || op.Kind == OpRename
		}
	}
	slices.Sort(pkgs)
	return slices.Compact(pkgs), importers
}

// pathScope is the packages that may declare path's root (API.md P6, P7, P7a).
func pathScope(units []*project.Unit, path string) []string {
	p, err := Parse(path)
	if err != nil {
		return nil
	}
	return declaring(units, p.Package, p.Root, isValueRoot)
}

// nameScope is the packages that may declare a RenameName's target (API.md E27): the root word's,
// with its imports when a member is named through a type, maybe another package's alias; for
// `file:line:col`, the file's package and its imports, whose declarations the identifier may name.
func nameScope(units []*project.Unit, name string) []string {
	if p, ok := splitPosition(name); ok {
		i := slices.IndexFunc(units, func(u *project.Unit) bool {
			return slices.ContainsFunc(u.Files, func(f *syntax.File) bool { return f.Src.Path == p.file })
		})
		if i < 0 {
			return nil
		}
		return importedBy(units, units[i].Name)
	}
	pkg, words, ok := splitNames(name)
	if !ok {
		return nil
	}
	out := declaring(units, pkg, words[0], isNameRoot)
	if len(words) == 1 || len(declaring(units, pkg, words[0], isTypeDecl)) == 0 {
		return out
	}
	for _, root := range out {
		out = append(out, importedBy(units, root)...)
	}
	return out
}

// declaring is the packages with a top-level declaration named word that kind takes: package pkg's
// alone when given, any of it; else every package's public one (API.md P6, P7, E27).
func declaring(units []*project.Unit, pkg, word string, kind func(syntax.Decl) bool) []string {
	var out []string
	for _, u := range units {
		if pkg != "" && u.Name != pkg {
			continue
		}
		if declares(u, word, pkg != "", kind) {
			out = append(out, u.Name)
		}
	}
	return out
}

// declares reports a declaration of u named word that kind takes, public unless all count.
func declares(u *project.Unit, word string, all bool, kind func(syntax.Decl) bool) bool {
	for _, f := range u.Files {
		for _, d := range f.Decls {
			name, mods := declName(d)
			if name != nil && name.Name == word && kind(d) && (all || mods == nil || !mods.Local.Valid()) {
				return true
			}
		}
	}
	return false
}

// declName is a top-level declaration's name and modifiers, nil for one with no name.
func declName(d syntax.Decl) (*syntax.Ident, *syntax.Modifiers) {
	switch d := d.(type) {
	case *syntax.LetDecl:
		return d.Name, d.Mods
	case *syntax.ConstDecl:
		return d.Name, d.Mods
	case *syntax.EnumDecl:
		return d.Name, d.Mods
	case *syntax.RecordDecl:
		return d.Name, d.Mods
	case *syntax.VariantDecl:
		return d.Name, d.Mods
	case *syntax.TypeDecl:
		return d.Name, d.Mods
	case *syntax.FnDecl:
		return d.Name, d.Mods
	}
	return nil, nil
}

// isValueRoot reports a declaration a path's root names: a let, a const or an enum (API.md P7a).
func isValueRoot(d syntax.Decl) bool {
	switch d.(type) {
	case *syntax.LetDecl, *syntax.ConstDecl, *syntax.EnumDecl:
		return true
	}
	return false
}

// isNameRoot reports a declaration a name's first word names: a type, a function, a let or a
// const (API.md E27, topName).
func isNameRoot(d syntax.Decl) bool {
	switch d.(type) {
	case *syntax.LetDecl, *syntax.ConstDecl, *syntax.EnumDecl, *syntax.RecordDecl, *syntax.VariantDecl,
		*syntax.TypeDecl, *syntax.FnDecl:
		return true
	}
	return false
}

// isTypeDecl reports a `type` declaration, which may name another package's type.
func isTypeDecl(d syntax.Decl) bool {
	_, ok := d.(*syntax.TypeDecl)
	return ok
}

// importedBy is pkg and every package it imports, directly or not.
func importedBy(units []*project.Unit, pkg string) []string {
	out := []string{pkg}
	for i := 0; i < len(out); i++ {
		j := slices.IndexFunc(units, func(u *project.Unit) bool { return u.Name == out[i] })
		if j < 0 {
			continue
		}
		for _, imp := range units[j].Imports {
			if !slices.Contains(out, imp) {
				out = append(out, imp)
			}
		}
	}
	return out
}
