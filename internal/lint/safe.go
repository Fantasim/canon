package lint

import (
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/syntax"
)

// broken is what the checker found broken: the declarations, views and translation entries, and the packages holding them.
type broken struct {
	nodes    map[syntax.Node]bool
	packages map[string]bool
}

// brokenIn collects what prog's checker found broken (TYPES.md §1, API.md E32).
func brokenIn(prog *check.Program) broken {
	b := broken{nodes: map[syntax.Node]bool{}, packages: map[string]bool{}}
	info := prog.Info
	for obj := range info.Broken { //canon:unordered a set is built
		b.nodes[obj.Decl()] = true
		b.packages[obj.Pkg()] = true
	}
	for _, pkg := range prog.Packages {
		for _, f := range pkg.Files {
			b.markFile(info, pkg.Path, f)
		}
	}
	return b
}

// markFile adds the broken views and translation entries of f, a file of package pkg.
func (b broken) markFile(info *check.Info, pkg string, f *syntax.File) {
	for _, d := range f.Decls {
		if v, ok := d.(*syntax.ViewDecl); ok && info.BrokenViews[v] {
			b.nodes[v], b.packages[pkg] = true, true
		}
	}
	for _, e := range f.Entries {
		if info.BrokenTranslations[e] {
			b.nodes[e], b.packages[pkg] = true, true
		}
	}
}

// graph is the project's packages: who imports whom, over every unit the analysis read, and which are loaded.
type graph struct {
	importers map[string][]string
	loaded    map[string]bool
}

// graphOf reads the import edges of units and the packages prog loaded.
func graphOf(prog *check.Program, units []*project.Unit) graph {
	g := graph{importers: map[string][]string{}, loaded: map[string]bool{}}
	for _, u := range units {
		for _, imp := range u.Imports {
			g.importers[imp] = append(g.importers[imp], u.Name)
		}
	}
	for _, pkg := range prog.Packages {
		g.loaded[pkg.Path] = true
	}
	return g
}

// safe reports that a rename in pkg is one canon edit accepts and every output it can change is seen: each package
// importing pkg, however far, is loaded, and neither those nor pkg hold a broken declaration (API.md E32).
func (p *pass) safe(pkg string) bool {
	return p.closure(pkg, func(name string) bool { return !p.broken.packages[name] && p.graph.loaded[name] })
}

// importersLoaded reports that pkg and every package importing it, however far, is loaded: no output outside the analysis reads it.
func (p *pass) importersLoaded(pkg string) bool {
	return p.closure(pkg, func(name string) bool { return p.graph.loaded[name] })
}

// closure reports that ok holds for pkg and every package importing it, however far.
func (p *pass) closure(pkg string, ok func(name string) bool) bool {
	seen := map[string]bool{pkg: true}
	todo := []string{pkg}
	for len(todo) > 0 {
		name := todo[0]
		todo = todo[1:]
		if !ok(name) {
			return false
		}
		for _, imp := range p.graph.importers[name] {
			if !seen[imp] {
				seen[imp] = true
				todo = append(todo, imp)
			}
		}
	}
	return true
}
