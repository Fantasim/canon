package check

import (
	"cmp"
	"context"
	"maps"
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
)

// kept is the journal Recheck starts from: the findings and folds it keeps as they were, and
// Broken and BrokenViews before propagation less the swapped files' declarations. It holds
// findings back until Recheck succeeds.
func (j *journal) kept(pl *recheckPlan) *journal {
	out := &journal{hold: true, views: maps.Clone(j.views)}
	out.direct = maps.Clone(j.direct)
	maps.DeleteFunc(out.direct, func(o Object, _ bool) bool { return pl.dropped(o) })
	for _, f := range j.findings {
		if !pl.redoes(f.from) {
			out.findings = append(out.findings, f)
		}
	}
	at := j.keptBreaks(pl, out)
	for _, fc := range j.folds {
		if !pl.dropped(fc.owner) {
			fc.breaks = at[fc.breaks]
			out.folds = append(out.folds, fc)
		}
	}
	return out
}

// keptBreaks puts in out the breaks of objects pl keeps; the result maps a count of j's breaks
// to the count of out's among them, for the folds' positions.
func (j *journal) keptBreaks(pl *recheckPlan, out *journal) []int {
	at := make([]int, len(j.breaks)+1)
	for i, o := range j.breaks {
		at[i+1] = at[i]
		if !pl.dropped(o) {
			out.breaks = append(out.breaks, o)
			at[i+1]++
		}
	}
	return at
}

// replayFolds refolds the kept folds for their findings and steps, in cold's order, before stage E's (DECISIONS 104).
func (c *checker) replayFolds(ctx context.Context, fold Folder, folds []foldCall) {
	whole := context.WithoutCancel(ctx) // the result is final: its folds' findings are part of it
	// The swapped files' own breaks are left out: no Fold reads an entry's Broken, as a let stops
	// at nonConstant before force (eval/names.go:115) and an ObjEntry is a Ref unread (names.go:73).
	final := c.info.Broken
	defer func() { c.info.Broken = final }()
	c.info.Broken = map[Object]bool{} // stage E's Info, Broken as each fold first saw it: one evaluator, one counter
	next := 0
	for _, fc := range folds { // swapped files never fold (entriesOnly, keepable): this is cold's order
		for ; next < fc.breaks; next++ {
			c.info.Broken[c.journal.breaks[next]] = true
		}
		fold.Fold(whole, fc.owner, fc.e, c.info)
	}
}

// swapFiles puts each new file in place of the old one: the old nodes leave Info, the new
// header and entries are bound, and every reference to an old entry moves to its new object;
// the result maps each old entry object to its new one.
func (c *checker) swapFiles(pl *recheckPlan) map[*object]*object {
	for _, sw := range pl.swaps {
		c.forget(sw.old)
		sw.pkg.files[slices.Index(sw.pkg.files, sw.old)] = sw.new
		if i := slices.Index(c.inputs, sw.old); i >= 0 {
			c.inputs[i] = sw.new
		}
	}
	c.useBags()
	renew := map[*object]*object{}
	for _, sw := range pl.swaps {
		c.bindFile(sw, renew)
	}
	c.renew(pl, renew)
	return renew
}

// forget drops what the checker recorded about the nodes of f.
func (c *checker) forget(f *syntax.File) {
	syntax.Inspect(f, func(n syntax.Node) bool {
		if n != nil {
			c.info.forget(n)
			delete(c.syntaxHeld, n)
			delete(c.badLits, n)
			delete(c.unmatchable, n)
		}
		return true
	})
}

// useBags points each package at its bag in c.bags, a new one added for a package without.
func (c *checker) useBags() {
	var fallback *fileIndex
	for _, p := range c.sorted {
		p.bag = c.bags[p.path]
		if p.bag == nil {
			fallback = cmp.Or(fallback, indexFiles(c.inputs))
			p.bag = diag.NewBag(fallback, p.path)
			c.bags[p.path] = p.bag
		}
	}
}

// rebindImports binds the new file's imports, its import edges where the old file's stood.
func (c *checker) rebindImports(p *pkgState, sw swap) {
	delete(p.scopes, sw.old)
	at := slices.IndexFunc(p.edges, func(e importEdge) bool { return e.file == sw.old })
	p.edges = slices.DeleteFunc(p.edges, func(e importEdge) bool { return e.file == sw.old })
	n := len(p.edges)
	c.bindFileImports(p, sw.new)
	if at >= 0 {
		added := slices.Clone(p.edges[n:])
		p.edges = slices.Insert(p.edges[:n], at, added...)
	}
}

// bindFile binds a new file's syntax errors, imports, directory and entries, each entry's
// object taking its old one's place.
func (c *checker) bindFile(sw swap, renew map[*object]*object) {
	p, nf := sw.pkg, sw.new
	c.syntaxErrorsIn(nf, c.syntaxSpans([]*pkgState{p}))
	c.rebindImports(p, sw)
	c.checkDir(p, nf)
	for i, d := range nf.Decls {
		e, old := d.(*syntax.EntryDecl), sw.objs[i]
		o := c.newObject(ObjEntry, entryKeyText(e.Key), p, e, nf)
		renew[old] = o
		p.all[slices.Index(p.all, old)] = o
		k := slices.IndexFunc(p.entries, func(at entryAt) bool { return at.obj == old })
		p.entries[k] = entryAt{decl: e, file: nf, obj: o}
		if table := c.registerEntry(p, p.entries[k]); table != nil {
			table.keys.replace(old, o)
		}
	}
}

// renew moves every reference to an old entry object to its new one: dependencies, E3101's
// duplicates and the unchanged files' Info; the old entries' key writes go.
func (c *checker) renew(pl *recheckPlan, renew map[*object]*object) {
	for old := range renew { //canon:unordered deletions
		delete(c.deps, old)
	}
	for _, ds := range c.deps { //canon:unordered each list is renewed in place
		for i, d := range ds {
			ds[i] = cmp.Or(renew[d], d)
		}
	}
	for _, p := range c.sorted {
		if !pl.pkgs[p.path] {
			continue
		}
		for i, d := range c.entryDups[p] {
			c.entryDups[p][i] = entryDup{entry: cmp.Or(renew[d.entry], d.entry), table: d.table, first: cmp.Or(renew[d.first], d.first)}
		}
		for _, o := range p.all {
			if w := c.keyedWrites[o]; w != nil {
				w.writes = slices.DeleteFunc(w.writes, func(kw keyWrite) bool { return renew[kw.entry] != nil })
			}
		}
	}
	c.info.renew(renew)
}

// objectsOf are the entry objects of a swap's new file, in order.
func (c *checker) objectsOf(sw swap) []*object {
	var out []*object
	for _, at := range sw.pkg.entries {
		if at.file == sw.new {
			out = append(out, at.obj)
		}
	}
	return out
}

// redoKeys reports again E3101 and E3102 in the swapped files' packages (TYPES.md §9.3).
func (c *checker) redoKeys(pl *recheckPlan) {
	for _, p := range c.sorted {
		if !pl.pkgs[p.path] {
			continue
		}
		for _, d := range c.entryDups[p] {
			c.duplicateEntry(p, d)
		}
		for _, o := range p.all {
			c.writtenTwice(o)
		}
	}
}

// renewPackages gives every package a new Package of its files and declarations, each old
// entry object replaced by its new one: the old program's stay as they were.
func (c *checker) renewPackages(renew map[*object]*object) {
	for _, p := range c.sorted {
		decls := slices.Clone(p.pkg.Decls)
		for i, o := range decls {
			if n := renewed(renew, o); n != nil {
				decls[i] = n
			}
		}
		p.pkg = &Package{Path: p.path, Files: slices.Clone(p.files), Decls: decls, Layers: p.pkg.Layers}
	}
}

// replace puts o in old's place among the keys, when old has one.
func (k *entryKeys) replace(old, o *object) {
	if k.byName[old.name] == old {
		k.byName[old.name] = o
	}
	if i := slices.Index(k.order, old); i >= 0 {
		k.order[i] = o
	}
}
