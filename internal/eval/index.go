package eval

import (
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
)

// index locates what the checked program leaves implicit: the package of each file, the file
// of each record, variant, function, check, test and where predicate, the record or variant
// declaring each check, the entry declarations of each let, and the broken declarations.
type index struct {
	pkg     map[*syntax.File]string
	file    map[syntax.Node]*syntax.File
	owner   map[*syntax.CheckDecl]syntax.Node
	entries map[check.Object][]check.Object
	decls   map[syntax.Node]check.Object
	broken  map[syntax.Node]bool
	written *writtenIndex
	files   []*syntax.File // walked for file and owner on the first lookup (fileOf)
	srcs    fileSet        // the files' sources by id, one table every bag shares (files); add extends it
	walked  bool
}

func emptyIndex() *index {
	return &index{
		pkg:     map[*syntax.File]string{},
		file:    map[syntax.Node]*syntax.File{},
		owner:   map[*syntax.CheckDecl]syntax.Node{},
		entries: map[check.Object][]check.Object{},
		decls:   map[syntax.Node]check.Object{},
		broken:  map[syntax.Node]bool{},
		srcs:    fileSet{},
	}
}

func buildIndex(prog *check.Program) *index {
	x := emptyIndex()
	for _, pkg := range prog.Packages {
		for _, f := range pkg.Files {
			x.pkg[f] = pkg.Path
			x.srcs[f.Src.ID] = f.Src
			x.files = append(x.files, f)
		}
		x.addDecls(pkg, prog.Info)
	}
	x.addBroken(prog.Info)
	x.written = &writtenIndex{prog: prog}
	return x
}

// addBroken records the declaration of each broken object, with the checks of records and variants no Decls lists (TYPES.md §1).
func (x *index) addBroken(info *check.Info) {
	if info == nil {
		return
	}
	for obj, broken := range info.Broken { //canon:unordered a set of declarations, read by lookup only
		if broken && obj != nil {
			x.broken[obj.Decl()] = true
		}
	}
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

// fileIndex is what one file adds to the index: the declarations and where predicates later
// lookups start from, and the record or variant declaring each check.
type fileIndex struct {
	decls  []syntax.Node
	checks []*syntax.CheckDecl
	owners []syntax.Node
}

// indexFile walks f for its fileIndex.
func indexFile(f *syntax.File) fileIndex {
	var fi fileIndex
	syntax.Inspect(f, func(n syntax.Node) bool {
		switch d := n.(type) {
		case *syntax.RecordDecl, *syntax.VariantDecl, *syntax.CheckDecl, *syntax.FnDecl,
			*syntax.TestDecl, *syntax.AmendBlock:
			fi.decls = append(fi.decls, d)
		case *syntax.WhereType:
			fi.decls = append(fi.decls, d.Pred)
		}
		return true
	})
	for _, d := range f.Decls {
		switch d.(type) {
		case *syntax.RecordDecl, *syntax.VariantDecl:
			syntax.Inspect(d, func(n syntax.Node) bool {
				if c, ok := n.(*syntax.CheckDecl); ok {
					fi.checks, fi.owners = append(fi.checks, c), append(fi.owners, d)
				}
				return true
			})
		}
	}
	return fi
}

// walkFiles fills the index's file and owner once, from each file's fileIndex, which a memo
// keeps across the evaluators of its epoch, and the per-file memo across all (an edit's host).
func (e *Evaluator) walkFiles() {
	x := e.index
	if x.walked {
		return
	}
	x.walked = true
	for _, f := range x.files {
		fi := cachedFile(e.memo, filesIndexed, f, indexedFiles.of)
		for _, n := range fi.decls {
			x.file[n] = f
		}
		for i, c := range fi.checks {
			x.owner[c] = fi.owners[i]
		}
	}
}

// add indexes f, a file of pkg a fold or a value met: its declarations are found on the next lookup.
func (x *index) add(f *syntax.File, pkg string) {
	if _, known := x.pkg[f]; known || f == nil {
		return
	}
	x.pkg[f], x.walked = pkg, false
	x.srcs[f.Src.ID] = f.Src
	x.files = append(x.files, f)
}

// fileOf is the file declaring n: a record, variant, check, function, test, amend block or where predicate.
func (e *Evaluator) fileOf(n syntax.Node) *syntax.File {
	e.walkFiles()
	return e.index.file[n]
}

// ownerOf is the record or variant declaring check c.
func (e *Evaluator) ownerOf(c *syntax.CheckDecl) syntax.Node {
	e.walkFiles()
	return e.index.owner[c]
}
