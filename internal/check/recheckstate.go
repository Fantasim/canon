package check

import (
	"cmp"
	"context"
	"maps"
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
)

// kept is the journal Recheck starts from, findings held back until it succeeds: the findings and
// folds it keeps (a let checked again keeps its signature's), and Broken and BrokenViews before
// propagation less the declarations checked again, but for the breaks of the folds it keeps.
func (j *journal) kept(pl *recheckPlan) *journal {
	out := &journal{hold: true, views: maps.Clone(j.views), byFold: maps.Clone(j.byFold)}
	out.direct = maps.Clone(j.direct)
	maps.DeleteFunc(out.direct, func(o Object, _ bool) bool { return !j.keepsBreak(pl, o) })
	for _, f := range j.findings {
		if !pl.redoes(f) {
			out.findings = append(out.findings, f)
		}
	}
	at := j.keptBreaks(pl, out)
	for _, fc := range j.folds {
		if !pl.dropped(fc.owner) || pl.sig[fc.e] != nil {
			fc.breaks = at[fc.breaks]
			out.folds = append(out.folds, fc)
		}
	}
	return out
}

// keptBreaks puts in out the breaks pl keeps; the result maps a count of j's breaks to the count
// of out's among them, for the folds' positions.
func (j *journal) keptBreaks(pl *recheckPlan, out *journal) []int {
	at := make([]int, len(j.breaks)+1)
	for i, o := range j.breaks {
		at[i+1] = at[i]
		if j.keepsBreak(pl, o) {
			out.breaks = append(out.breaks, o)
			at[i+1]++
		}
	}
	return at
}

// keepsBreak reports a break Recheck keeps: an object it keeps, or one a failed fold broke, which
// it keeps only with the fold (keepable): the replayed fold fails as cold's, which breaks the let
// before its value is checked (DECISIONS 150, 263).
func (j *journal) keepsBreak(pl *recheckPlan, o Object) bool {
	return !pl.dropped(o) || j.byFold[o]
}

// replayFolds refolds the kept folds for their findings and steps, in cold's order, before stage E's (DECISIONS 104).
func (c *checker) replayFolds(ctx context.Context, fold Folder, folds []foldCall) {
	whole := context.WithoutCancel(ctx) // the result is final: its folds' findings are part of it
	// The invariant: no fold reads an object broken at its time (checkBounds folds a bound only once
	// readsBroken finds none of the objects it names broken), so replaying the kept breaks, and not
	// the swapped files' own, gives each fold the answer cold gave it.
	final := c.info.Broken
	defer func() { c.info.Broken = final }()
	c.info.Broken = map[Object]bool{} // stage E's Info, Broken as each fold first saw it: one evaluator, one counter
	next := 0
	for _, fc := range folds { // cold's order; apply refuses a swapped file that folds
		for ; next < fc.breaks; next++ {
			c.info.Broken[c.journal.breaks[next]] = true
		}
		fold.Fold(whole, fc.owner, fc.e, c.info)
	}
}

// swapFiles puts each new file in place of the old one: the old nodes it does not share leave
// Info, the new header, entries and lets are bound, and every reference moves to its renewed
// object; the result maps each renewed object to its new one.
func (c *checker) swapFiles(pl *recheckPlan) map[*object]*object {
	c.moveSignatures(pl)
	imported := make([][]*object, len(pl.swaps))
	for i, sw := range pl.swaps {
		imported[i] = c.importObjects(sw)
		c.forget(sw.old, sw.shared)
		sw.pkg.files[slices.Index(sw.pkg.files, sw.old)] = sw.new
		if i := slices.Index(c.inputs, sw.old); i >= 0 {
			c.inputs[i] = sw.new
		}
	}
	c.useBags()
	renew := map[*object]*object{}
	for i, sw := range pl.swaps {
		c.bindFile(sw, imported[i], renew)
	}
	if pl.wide {
		c.renewWide(pl, renew)
	} else {
		c.renew(pl, renew)
	}
	return renew
}

// forget drops what the checker recorded about the nodes of f but those the new version shares.
func (c *checker) forget(f *syntax.File, shared map[syntax.Node]bool) {
	syntax.Inspect(f, func(n syntax.Node) bool {
		if n == nil || shared[n] {
			return false
		}
		c.info.forget(n)
		delete(c.syntaxHeld, n)
		delete(c.badLits, n)
		delete(c.unmatchable, n)
		delete(c.unrefined, n)
		return true
	})
}

// moveSignatures gives the new nodes of each let checked again the facts of its signature's old
// ones: Recheck keeps its resolved type, so no type or ref is made again (NFR-02).
func (c *checker) moveSignatures(pl *recheckPlan) {
	for old, n := range pl.sig { //canon:unordered each node's facts move on their own
		c.info.move(old, n)
		moveMark(c.syntaxHeld, old, n)
		moveMark(c.badLits, old, n)
		moveMark(c.unmatchable, old, n)
		moveMark(c.unrefined, old, n)
	}
}

// moveMark copies a node's mark to its new node.
func moveMark(m map[syntax.Node]bool, old, n syntax.Node) {
	if v, ok := m[old]; ok {
		m[n] = v
	}
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

// bindFile binds a new file's syntax errors, imports, directory, entries and lets checked
// again, each new object taking its old one's place; a shared declaration keeps its facts.
func (c *checker) bindFile(sw swap, imported []*object, renew map[*object]*object) {
	p, nf := sw.pkg, sw.new
	c.syntaxErrorsIn(nf, c.syntaxSpans([]*pkgState{p}))
	c.rebindImports(p, sw)
	c.renewImports(sw, imported, renew)
	c.checkDir(p, nf)
	for _, d := range sw.pairs {
		switch x := d.new.(type) {
		case *syntax.EntryDecl:
			if !d.shared() {
				c.bindEntry(p, nf, x, d.obj, renew)
			}
		case *syntax.LetDecl:
			if !d.shared() {
				c.bindLet(p, nf, x, d, renew)
			}
		}
	}
}

// bindEntry makes the object of an `entry` declaration checked again, in old's place.
func (c *checker) bindEntry(p *pkgState, nf *syntax.File, e *syntax.EntryDecl, old *object, renew map[*object]*object) {
	o := c.newObject(ObjEntry, entryKeyText(e.Key), p, e, nf)
	renew[old] = o
	p.all[slices.Index(p.all, old)] = o
	k := slices.IndexFunc(p.entries, func(at entryAt) bool { return at.obj == old })
	p.entries[k] = entryAt{decl: e, file: nf, obj: o}
	if table := c.registerEntry(p, p.entries[k]); table != nil {
		table.keys.replace(old, o)
	}
}

// bindLet makes the object of a let checked again, with its old one's resolved type and keys,
// and the objects of its table literal's rows, each in its old one's place.
func (c *checker) bindLet(p *pkgState, nf *syntax.File, d *syntax.LetDecl, pair declPair, renew map[*object]*object) {
	old := pair.obj
	o := c.newObject(ObjLet, old.name, p, d, nf)
	o.local, o.mutable, o.typ, o.state, o.keys = old.local, old.mutable, old.typ, stateDone, old.keys
	renew[old] = o
	lit, ok := d.Value.(*syntax.BraceLit)
	if !ok || len(pair.rows) == 0 {
		return
	}
	for i, it := range lit.Items {
		e := it.(*syntax.EntryItem)
		row := c.newObject(ObjEntry, e.Key.Name, p, e, nf)
		row.parent = o
		c.info.Defs[e.Key] = row
		renew[pair.rows[i]] = row
	}
}

// importObjects are the package objects the old version's imports bound, by index: read before
// forget, which drops an unshared header's.
func (c *checker) importObjects(sw swap) []*object {
	out := make([]*object, len(sw.old.Imports))
	for i, imp := range sw.old.Imports {
		out[i], _ = c.info.NameUses[lastPart(imp)].(*object)
	}
	return out
}

// renewImports maps the package object each old import bound to the one its new import, written
// alike at the same index (sameHeader), binds.
func (c *checker) renewImports(sw swap, imported []*object, renew map[*object]*object) {
	for i, imp := range sw.new.Imports {
		n, ok := c.info.NameUses[lastPart(imp)].(*object)
		if old := imported[i]; ok && old != nil && n != old {
			renew[old] = n
		}
	}
}

// lastPart is the identifier an import's package object is bound at: its path's last part.
func lastPart(imp *syntax.Import) *syntax.Ident {
	return imp.Path.Parts[len(imp.Path.Parts)-1]
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
	c.info.renew(byMap(renew))
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
// object replaced by its new one: the old program's stay as they were.
func (c *checker) renewPackages(to renewal) {
	for _, p := range c.sorted {
		decls := slices.Clone(p.pkg.Decls)
		for i, o := range decls {
			if n := renewed(to, o); n != nil {
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
