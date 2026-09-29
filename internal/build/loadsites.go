package build

import (
	"sync"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// loadAt is a load expression of a file, with its span.
type loadAt struct {
	e    *syntax.LoadExpr
	span source.Span
}

// loadSites is where each load expression of a program was written, every file walked once on
// the first question; with a cache, a file an earlier snapshot walked is not walked again.
type loadSites struct {
	prog  *check.Program
	gen   *cacheGen
	once  sync.Once
	sites map[*syntax.LoadExpr]loadSite
}

// loadSites is the load sites of the run's program.
func (r *run) loadSites() *loadSites {
	return &loadSites{prog: r.prog, gen: r.s.gen}
}

// find is e's site, false when no file of the program holds it.
func (l *loadSites) find(e *syntax.LoadExpr) (loadSite, bool) {
	l.once.Do(l.index)
	site, ok := l.sites[e]
	return site, ok
}

// index records the site of every load expression of the program.
func (l *loadSites) index() {
	l.sites = map[*syntax.LoadExpr]loadSite{}
	var files []*syntax.File
	for _, cp := range l.prog.Packages {
		for _, f := range cp.Files {
			files = append(files, f)
			for _, at := range l.gen.loadsOf(f) {
				l.sites[at.e] = loadSite{pkg: cp.Path, file: f, span: at.span}
			}
		}
	}
	l.gen.keepLoads(files)
}

// loadsOf is every load expression of f, as the generation keeps it; walked without a cache.
func (g *cacheGen) loadsOf(f *syntax.File) []loadAt {
	if g == nil {
		return walkLoads(f)
	}
	g.mu.Lock()
	kept, ok := g.sites[f]
	g.mu.Unlock()
	if ok {
		return kept
	}
	found := walkLoads(f)
	g.mu.Lock()
	defer g.mu.Unlock()
	g.sites[f] = found
	return found
}

// keepLoads forgets the walks of files a program no longer holds, once they outnumber its own.
func (g *cacheGen) keepLoads(files []*syntax.File) {
	if g == nil {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if len(g.sites) <= compactRatio*len(files) {
		return
	}
	live := make(map[*syntax.File]bool, len(files))
	for _, f := range files {
		live[f] = true
	}
	for f := range g.sites { //canon:unordered deleting the dead ones, in any order
		if !live[f] {
			delete(g.sites, f)
		}
	}
}

// walkLoads is every load expression of f, in walk order.
func walkLoads(f *syntax.File) []loadAt {
	var out []loadAt
	syntax.Inspect(f, func(n syntax.Node) bool {
		if e, ok := n.(*syntax.LoadExpr); ok {
			out = append(out, loadAt{e: e, span: f.Span(e)})
		}
		return true
	})
	return out
}

// siteIn is e's site in prog, through the host's sites when they are prog's.
func (h *evalHost) siteIn(prog *check.Program, e *syntax.LoadExpr) (loadSite, bool) {
	if h.sites == nil || h.sites.prog != prog {
		h.sites = &loadSites{prog: prog}
	}
	return h.sites.find(e)
}
