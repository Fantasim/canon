package eval

import (
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
)

// index locates what the checked program leaves implicit: the package of each file, the file
// of each record, variant, function, check, test and where predicate, the record or variant
// declaring each check, and the entry declarations of each let.
type index struct {
	pkg     map[*syntax.File]string
	file    map[syntax.Node]*syntax.File
	owner   map[*syntax.CheckDecl]syntax.Node
	entries map[check.Object][]check.Object
	decls   map[syntax.Node]check.Object
	written *writtenIndex
}

func emptyIndex() *index {
	return &index{
		pkg:     map[*syntax.File]string{},
		file:    map[syntax.Node]*syntax.File{},
		owner:   map[*syntax.CheckDecl]syntax.Node{},
		entries: map[check.Object][]check.Object{},
		decls:   map[syntax.Node]check.Object{},
	}
}

func buildIndex(prog *check.Program) *index {
	x := emptyIndex()
	for _, pkg := range prog.Packages {
		for _, f := range pkg.Files {
			x.pkg[f] = pkg.Path
			x.addFile(f)
		}
		x.addDecls(pkg, prog.Info)
	}
	x.written = &writtenIndex{prog: prog}
	return x
}

// addDecls records a package's declarations by node, and each `entry` under its let.
func (x *index) addDecls(pkg *check.Package, info *check.Info) {
	for _, obj := range pkg.Decls {
		x.decls[obj.Decl()] = obj
		d, ok := obj.Decl().(*syntax.EntryDecl)
		if !ok || obj.Kind() != check.ObjEntry {
			continue
		}
		if table := info.NameUses[d.Table]; table != nil {
			x.entries[table] = append(x.entries[table], obj)
		}
	}
}

// addFile records the declarations of f that later lookups start from.
func (x *index) addFile(f *syntax.File) {
	syntax.Inspect(f, func(n syntax.Node) bool {
		switch d := n.(type) {
		case *syntax.RecordDecl, *syntax.VariantDecl, *syntax.CheckDecl, *syntax.FnDecl,
			*syntax.TestDecl, *syntax.AmendBlock:
			x.file[d] = f
		case *syntax.WhereType:
			x.file[d.Pred] = f
		}
		return true
	})
	for _, d := range f.Decls {
		switch d.(type) {
		case *syntax.RecordDecl, *syntax.VariantDecl:
			syntax.Inspect(d, func(n syntax.Node) bool {
				if c, ok := n.(*syntax.CheckDecl); ok {
					x.owner[c] = d
				}
				return true
			})
		}
	}
}
