package edit

import (
	"sync"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
)

// declIndex is what a snapshot's lookups read, built once at the first: without it each value a
// path reads walks every declaration of the program for its root and its collection's entries.
type declIndex struct {
	once    sync.Once
	roots   map[string][]rootRef      // every let, const and enum by name, package then declaration order
	entries map[check.Object][]item   // each collection's entry declarations, in that order (W2)
	sites   map[source.Span]entrySite // each entry declaration by its span
}

// entrySite is an entry declaration and the collection it adds to.
type entrySite struct {
	it    item
	table check.Object
}

// decls is s's declaration index, built on first use.
func (s *Snapshot) decls() *declIndex {
	s.index.once.Do(s.buildIndex)
	return &s.index
}

func (s *Snapshot) buildIndex() {
	x := &s.index
	x.roots, x.entries, x.sites = map[string][]rootRef{}, map[check.Object][]item{}, map[source.Span]entrySite{}
	for _, pkg := range s.pkgs {
		for _, obj := range pkg.Decls {
			if r, ok := rootOf(pkg, obj, obj.Name()); ok {
				x.roots[obj.Name()] = append(x.roots[obj.Name()], r)
			}
			d, ok := obj.Decl().(*syntax.EntryDecl)
			if !ok || obj.Kind() != check.ObjEntry {
				continue
			}
			it, table := item{d, obj.File()}, s.info.NameUses[d.Table]
			x.entries[table] = append(x.entries[table], it)
			x.sites[it.file.Span(d)] = entrySite{it, table}
		}
	}
}

// entryAt is the entry declaration of table whose span is at.
func (s *Snapshot) entryAt(table check.Object, at source.Span) (item, bool) {
	site, ok := s.decls().sites[at]
	return site.it, ok && site.table == table
}

// tableKeys is, per table read, the index of its first entry under each string key, built at
// its first read: a path read under each entry of a large table would scan it every time.
type tableKeys struct {
	mu sync.Mutex
	by map[*value.Table]map[string]int
}

// find is the index of t's first entry under the string key name; without an index (a layer
// line's walk, which has no snapshot) t is scanned.
func (k *tableKeys) find(t *value.Table, name string) (int, bool) {
	if k == nil {
		for i, e := range t.Entries {
			if !e.Ident.Key.IsInt && e.Ident.Key.S == name {
				return i, true
			}
		}
		return 0, false
	}
	i, ok := k.of(t)[name]
	return i, ok
}

// of is t's key index, built at its first read.
func (k *tableKeys) of(t *value.Table) map[string]int {
	k.mu.Lock()
	defer k.mu.Unlock()
	if idx, ok := k.by[t]; ok {
		return idx
	}
	idx := make(map[string]int, len(t.Entries))
	for i, e := range t.Entries {
		if _, seen := idx[e.Ident.Key.S]; !seen && !e.Ident.Key.IsInt {
			idx[e.Ident.Key.S] = i
		}
	}
	if k.by == nil {
		k.by = map[*value.Table]map[string]int{}
	}
	k.by[t] = idx
	return idx
}
