package project

import (
	"cmp"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// Tiers is the `tiers` key: the tiers lowest first, the key's span, each listed path's tier (GRAMMAR.md §7.1).
type Tiers struct {
	List []Tier
	Span source.Span
	at   map[string]int
}

// Tier is one tier: its name, the name's span and the package paths it lists, in written order.
type Tier struct {
	Name  string
	Span  source.Span
	Paths []Package
}

// Declared reports a tier declared: `tiers {}` declares none (DECISIONS 380).
func (t Tiers) Declared() bool {
	return len(t.List) > 0
}

// Of is pkg's tier and the longest listed path at or above it, cut at segments (GRAMMAR.md §7.1).
func (t Tiers) Of(pkg string) (tier int, path string, ok bool) {
	for p := pkg; p != ""; {
		if i, found := t.at[p]; found {
			return i, p, true
		}
		cut := strings.LastIndex(p, nameSep)
		if cut < 0 {
			break
		}
		p = p[:cut]
	}
	return 0, "", false
}

// tierReader reads one `tiers` map: the names given and each path's first span.
type tierReader struct {
	*schema
	names map[string]bool
	first map[string]source.Span
}

// tiers is an ordered map of tier names to non-empty lists of package paths (E1005, E1006, E1017).
func (s *schema) tiers(e *syntax.ProjectEntry) {
	m, ok := e.Value.(*syntax.ProjectMap)
	if !ok {
		s.fail(diag.E1006.AtTiers(s.span(e.Value)))
		return
	}
	s.p.Tiers = Tiers{Span: s.span(e.Key), at: map[string]int{}}
	r := &tierReader{schema: s, names: map[string]bool{}, first: map[string]source.Span{}}
	for _, g := range m.Entries {
		if !isBad(g.Key) && !isBad(g.Value) {
			r.entry(g)
		}
	}
}

// entry is one tier: an identifier name given once (E1017 name, E1005 tier), its paths (E1006).
func (r *tierReader) entry(g *syntax.ProjectEntry) {
	name, ident := r.keyName(g.Key)
	span := r.span(g.Key)
	switch {
	case !ident:
		r.fail(diag.E1017.AtName(span, name))
	case r.names[name]:
		r.fail(diag.E1005.AtTier(span, name))
	}
	r.names[name] = true
	l, ok := g.Value.(*syntax.ProjectList)
	if !ok || len(l.Items) == 0 {
		r.fail(diag.E1006.AtTiers(r.span(g.Value)))
		return
	}
	t := Tier{Name: name, Span: span}
	for _, item := range l.Items {
		if p, ok := r.path(item); ok {
			t.Paths = append(t.Paths, p)
		}
	}
	tiers := &r.p.Tiers
	for _, p := range t.Paths {
		tiers.at[p.Path] = len(tiers.List)
	}
	tiers.List = append(tiers.List, t)
}

// path is one listed package path: a dotted identifier (E1006), listed once over all tiers (E1017 duplicate).
func (r *tierReader) path(item syntax.ProjectValue) (Package, bool) {
	q, ok := item.(*syntax.QualifiedName)
	span := r.span(item)
	path := qualified(q)
	first, seen := r.first[path]
	switch {
	case isBad(item):
	case !ok:
		r.fail(diag.E1006.AtTiers(span))
	case seen:
		r.fail(diag.E1017.AtDuplicate(span, path, first))
	default:
		r.first[path] = span
		return Package{Path: path, Span: span}, true
	}
	return Package{}, false
}

// CheckTiers reports a package no path places and a path placing none (E1018, GRAMMAR.md §7.1).
func CheckTiers(p *Project, units []*Unit, bag *diag.Bag) {
	t := p.Tiers
	if !t.Declared() {
		return
	}
	names := make([]string, 0, len(units))
	for _, u := range units {
		names = append(names, u.Name)
	}
	slices.Sort(names)
	used := map[string]bool{}
	for _, name := range names {
		if _, path, ok := t.Of(name); ok {
			used[path] = true
			continue
		}
		diag.E1018.AtUnplaced(t.Span, name).Report(bag)
	}
	for _, tier := range t.List {
		for _, path := range tier.Paths {
			if !used[path.Path] {
				diag.E1018.AtUnused(path.Span, path.Path).Report(bag)
			}
		}
	}
}

// CheckTierImports reports an import of units climbing the tiers; all is in name order (E2007, TYPES.md §3.1).
func CheckTierImports(p *Project, units, all []*Unit, bagOf func(string) *diag.Bag) {
	t := p.Tiers
	if !t.Declared() {
		return
	}
	c := tierImports{tiers: t, all: all, bagOf: bagOf}
	for _, u := range units {
		from, _, ok := t.Of(u.Name)
		if !ok {
			continue
		}
		for _, f := range u.Files {
			c.file(u.Name, from, f)
		}
	}
}

// tierImports judges imports against tiers: all are the packages, in name order; bagOf a package's bag.
type tierImports struct {
	tiers Tiers
	all   []*Unit
	bagOf func(string) *diag.Bag
}

// file judges the imports of one file of pkg, which tier from places.
func (c tierImports) file(pkg string, from int, f *syntax.File) {
	for _, imp := range f.Imports {
		if imp.Path == nil {
			continue
		}
		target := qualified(imp.Path)
		if _, found := slices.BinarySearchFunc(c.all, target, func(u *Unit, n string) int { return cmp.Compare(u.Name, n) }); !found {
			continue
		}
		to, _, ok := c.tiers.Of(target)
		if !ok || to <= from {
			continue
		}
		diag.E2007.At(f.Span(imp.Path), pkg, c.tiers.List[from].Name, target, c.tiers.List[to].Name).Report(c.bagOf(pkg))
	}
}
