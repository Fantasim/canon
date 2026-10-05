package check

import (
	"cmp"
	"path"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// pkgState is a package being checked.
type pkgState struct {
	path    string
	files   []*syntax.File // source, layer and translation files, in path order
	bag     *diag.Bag
	names   map[string]*object // the package namespace (TYPES.md §3.2)
	all     []*object          // every declaration that can be broken
	imports []*pkgState        // direct imports, in path order
	scopes  map[*syntax.File]*fileScope
	pkg     *Package
	entries []entryAt
	edges   []importEdge
	refKeys []refKey // ref key fields met while resolving, judged after it (DECISIONS 316)
	keyed   bool     // the ref keys of resolution are judged: a later one is judged at once
}

// entryAt is an `entry` declaration, its file and its object.
type entryAt struct {
	decl *syntax.EntryDecl
	file *syntax.File
	obj  *object
}

// fileScope is what a file's imports bind (TYPES.md §3.1: imports are per file).
type fileScope struct {
	names map[string]*object
	first map[string]source.Span
}

// loadPackages groups the files by their package clause, in path order.
func (c *checker) loadPackages(files []*syntax.File) {
	sorted := slices.Clone(files)
	slices.SortFunc(sorted, func(a, b *syntax.File) int { return cmp.Compare(a.Src.Path, b.Src.Path) })
	var fallback *fileIndex
	for _, f := range sorted {
		if f.Package == nil || f.FileKind == syntax.FileProject || f.FileKind == syntax.FileInvalid {
			continue
		}
		name := qualified(f.Package)
		p, ok := c.pkgs[name]
		if !ok {
			p = c.newPackage(name)
		}
		if p.bag == nil {
			fallback = cmp.Or(fallback, indexFiles(files))
			p.bag = diag.NewBag(fallback, name)
			c.bags[name] = p.bag
		}
		p.files = append(p.files, f)
		p.pkg.Files = append(p.pkg.Files, f)
	}
	for _, p := range c.sorted {
		c.checkDirs(p)
	}
}

func (c *checker) newPackage(name string) *pkgState {
	p := &pkgState{
		path:   name,
		bag:    c.bags[name],
		names:  map[string]*object{},
		scopes: map[*syntax.File]*fileScope{},
		pkg:    &Package{Path: name, Layers: map[string][]*syntax.AmendBlock{}},
	}
	c.pkgs[name] = p
	i, _ := slices.BinarySearchFunc(c.sorted, name, func(q *pkgState, n string) int { return cmp.Compare(q.path, n) })
	c.sorted = slices.Insert(c.sorted, i, p)
	return p
}

// qualified is a dotted name as written.
func qualified(q *syntax.QualifiedName) string {
	return syntax.Qualified(q)
}

// packageDir is the directory of a package: its path with dots as slashes.
func packageDir(pkg string) string { return strings.ReplaceAll(pkg, dot, slash) }

// fileDir is the directory of a file, "" at the project root.
func fileDir(f *syntax.File) string {
	if d := path.Dir(f.Src.Path); d != dot {
		return d
	}
	return ""
}

// checkDirs is E2006: a file declares its own directory's package or an ancestor's.
func (c *checker) checkDirs(p *pkgState) {
	for _, f := range p.files {
		c.checkDir(p, f)
	}
}

// checkDir is E2006 for one file of p.
func (c *checker) checkDir(p *pkgState, f *syntax.File) {
	want := packageDir(p.path)
	dir := fileDir(f)
	if dir == want || strings.HasPrefix(dir, want+slash) {
		return
	}
	if dir == "" {
		dir = rootDir
	}
	c.deliver(p, origin{file: f}, diag.E2006.At(f.Span(f.Package), dir, p.path).Report)
}

// fileIndex resolves the spans of the checked files, for a bag the caller did not give.
type fileIndex struct {
	byID map[source.FileID]*source.File
}

func indexFiles(files []*syntax.File) *fileIndex {
	x := &fileIndex{byID: map[source.FileID]*source.File{}}
	for _, f := range files {
		x.byID[f.Src.ID] = f.Src
	}
	return x
}

func (x *fileIndex) Path(id source.FileID) string {
	if f := x.byID[id]; f != nil {
		return f.Path
	}
	return ""
}

func (x *fileIndex) Position(id source.FileID, p source.Pos) (line, col int) {
	if f := x.byID[id]; f != nil {
		return f.Position(p)
	}
	return 0, 0
}

func (x *fileIndex) Content(id source.FileID) []byte {
	if f := x.byID[id]; f != nil {
		return f.Content
	}
	return nil
}
