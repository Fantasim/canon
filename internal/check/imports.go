package check

import (
	"cmp"
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// importEdge is one import of a file, for the cycle check (E2002).
type importEdge struct {
	to   *pkgState
	imp  *syntax.Import
	file *syntax.File
}

// bindImports binds the names each file of p imports (TYPES.md §3.1), E2003 to E2005.
func (c *checker) bindImports(p *pkgState) {
	for _, f := range p.files {
		c.bindFileImports(p, f)
	}
}

// bindFileImports binds the names one file of p imports.
func (c *checker) bindFileImports(p *pkgState, f *syntax.File) {
	fs := &fileScope{names: map[string]*object{}, first: map[string]source.Span{}}
	p.scopes[f] = fs
	for _, imp := range f.Imports {
		c.bindImport(p, f, fs, imp)
	}
}

func (c *checker) bindImport(p *pkgState, f *syntax.File, fs *fileScope, imp *syntax.Import) {
	last := imp.Path.Parts[len(imp.Path.Parts)-1]
	target, ok := c.pkgs[qualified(imp.Path)]
	if !ok {
		c.deliver(p, origin{file: f}, diag.E2003.At(f.Span(imp.Path), qualified(imp.Path)).Report)
		return
	}
	pkgObj := c.newObject(ObjPackage, last.Name, p, imp, f)
	pkgObj.target = target
	c.info.NameUses[last] = pkgObj
	p.edges = append(p.edges, importEdge{to: target, imp: imp, file: f})
	if !slices.Contains(p.imports, target) && target != p {
		p.imports = append(p.imports, target)
		slices.SortFunc(p.imports, byPath)
	}
	if len(imp.Names) > 0 {
		for _, n := range imp.Names {
			c.bindSelective(p, f, fs, target, n)
		}
		return
	}
	bound := last
	if imp.Alias != nil {
		bound = imp.Alias
		c.info.Defs[imp.Alias] = pkgObj
		pkgObj.name = imp.Alias.Name
	}
	c.bindName(p, f, fs, bound, pkgObj)
}

// bindSelective binds one name of `import p { A }`: a public declaration of p (E2004).
func (c *checker) bindSelective(p *pkgState, f *syntax.File, fs *fileScope, target *pkgState, n *syntax.Ident) {
	o, ok := target.names[n.Name]
	if !ok || o.local {
		c.deliver(p, origin{file: f}, diag.E2004.At(f.Span(n), target.path, n.Name).Report)
		return
	}
	c.info.NameUses[n] = o
	c.bindName(p, f, fs, n, o)
}

// bindName is E2005: a name bound twice in one file, or bound by an import and declared.
func (c *checker) bindName(p *pkgState, f *syntax.File, fs *fileScope, n *syntax.Ident, o *object) {
	if first, ok := fs.first[n.Name]; ok {
		c.deliver(p, origin{file: f}, diag.E2005.At(f.Span(n), n.Name, first).Report)
		return
	}
	if decl, ok := p.names[n.Name]; ok {
		c.deliver(p, origin{file: f}, diag.E2005.At(f.Span(n), n.Name, declSpan(decl)).Report)
		return
	}
	fs.first[n.Name] = f.Span(n)
	fs.names[n.Name] = o
}

// declSpan is the span of the identifier a declaration binds.
func declSpan(o *object) source.Span {
	if n := declName(o.decl); n != nil {
		return o.file.Span(n)
	}
	return o.file.Span(o.decl)
}

func byPath(a, b *pkgState) int { return cmp.Compare(a.path, b.path) }

// orderPackages sorts the packages dependencies first, ties by path (TYPES.md §3.1).
func (c *checker) orderPackages() {
	c.findCycles()
	placed := map[*pkgState]bool{}
	for len(c.order) < len(c.sorted) {
		next := c.nextReady(placed)
		placed[next] = true
		c.order = append(c.order, next)
	}
}

// nextReady is the first package by path whose imports are placed; on a cycle, the first left.
func (c *checker) nextReady(placed map[*pkgState]bool) *pkgState {
	var first *pkgState
	for _, p := range c.sorted {
		if placed[p] {
			continue
		}
		if first == nil {
			first = p
		}
		if !slices.ContainsFunc(p.imports, func(q *pkgState) bool { return !placed[q] }) {
			return p
		}
	}
	return first
}

// findCycles walks the import graph depth first and reports the import closing each cycle.
func (c *checker) findCycles() {
	state := map[*pkgState]visit{}
	var stack []*pkgState
	var walk func(p *pkgState)
	walk = func(p *pkgState) {
		state[p] = visiting
		stack = append(stack, p)
		for _, e := range p.edges {
			switch state[e.to] {
			case visiting:
				c.reportCycle(stack, e)
			case unvisited:
				walk(e.to)
			default:
			}
		}
		stack = stack[:len(stack)-1]
		state[p] = visited
	}
	for _, p := range c.sorted {
		if state[p] == unvisited {
			walk(p)
		}
	}
}

func (c *checker) reportCycle(stack []*pkgState, e importEdge) {
	from := slices.Index(stack, e.to)
	var names []string
	for _, p := range stack[from:] {
		names = append(names, p.path)
	}
	names = append(names, e.to.path)
	owner := stack[len(stack)-1]
	c.deliver(owner, origin{}, diag.E2002.At(e.file.Span(e.imp.Path), names).Report)
}

// visit is the state of a package in the cycle walk.
type visit uint8
