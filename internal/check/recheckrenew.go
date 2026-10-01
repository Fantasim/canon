package check

import (
	"maps"
	"slices"

	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// boundAt is where a bound is written, its file and range. A Recheck remaps a let signature's
// range to its new node, whose token indexes may have moved; the file is renewed by construction:
// the append-only file set still reads the old one's identical text at a shared range.
type boundAt struct {
	file *syntax.File
	node syntax.Node
}

// span is the bound's span, the zero span for a bound not recorded.
func (b boundAt) span() source.Span {
	if b.file == nil {
		return source.Span{}
	}
	return b.file.Span(b.node)
}

// renewer decides, once per object, its object in the program checked again: bindFile's new one,
// a copy with the new file (a shared declaration's, its type kept) or the new parent, or itself.
// The old program keeps its objects.
type renewer struct {
	pl *recheckPlan
	to map[*object]*object
}

// of is o's object in the program checked again.
func (r *renewer) of(o *object) *object {
	if o == nil {
		return nil
	}
	if n, ok := r.to[o]; ok {
		return n
	}
	if r.pl.old[o.file] == nil && o.parent == nil {
		return o
	}
	n := o
	if nf := r.pl.old[o.file]; nf != nil && !r.pl.redo[o] {
		cp := *o
		cp.file = nf
		n = &cp
	}
	r.to[o] = n
	if o.parent == nil {
		return n
	}
	if p := r.of(o.parent); p != o.parent {
		if n == o {
			cp := *o
			n = &cp
			r.to[o] = n
		}
		n.parent = p
	}
	return n
}

// settle maps each object bindFile replaced to its last renewal: an `entry` bound again hangs
// from its table's old object, which a swapped file renews, so it is renewed once more; every
// reference, Decls' and recheckDecls' too, then names one object, as cold (NFR-02).
func (r *renewer) settle() {
	for _, old := range slices.Collect(maps.Keys(r.to)) { // any order: a renewal reads only parents
		r.to[old] = r.of(r.to[old])
	}
}

// obj is of for an Object.
func (r *renewer) obj(o Object) Object {
	if x, ok := o.(*object); ok && x != nil {
		return r.of(x)
	}
	return o
}

// renewWide moves every reference the checker's state holds to its object in the program
// checked again: Info, the checker's maps, the record bodies, the packages, the recorded spans
// and the journal. What a declaration checked again recomputes is dropped instead.
func (c *checker) renewWide(pl *recheckPlan, renew map[*object]*object) {
	r := &renewer{pl: pl, to: renew}
	r.settle()
	c.info.renew(r.of)
	for k, o := range c.info.Defs { //canon:unordered each entry is renewed in place
		if n := renewed(r.of, o); n != nil {
			c.info.Defs[k] = n
		}
	}
	c.info.Broken = renewSet(c.info.Broken, r.obj)
	c.renewState(r)
	c.renewBodies(r)
	c.renewPackageState(r)
	c.renewSpans(pl)
	c.journal.renew(r, pl)
}

// recheckDecls checks again each entry and let a swap does not share, a keyed list's keys too.
func (c *checker) recheckDecls(pl *recheckPlan, renew map[*object]*object) {
	for _, sw := range pl.swaps {
		for _, d := range sw.pairs {
			if d.shared() {
				continue
			}
			o := renew[d.obj]
			c.checkDecl(o)
			if o.kind != ObjLet {
				continue
			}
			if keyed := letKeyed(o); keyed != nil && d.new.(*syntax.LetDecl).Value != nil {
				c.writtenKeys(o, keyed)
			}
		}
	}
}

// renewState renews the checker's maps by object.
func (c *checker) renewState(r *renewer) {
	c.deps = renewKeys(c.deps, r)
	for _, ds := range c.deps { //canon:unordered each list is renewed in place
		renewSlice(ds, r)
	}
	c.initDone = renewKeys(c.initDone, r)
	c.listKeys = renewKeys(c.listKeys, r)
	c.cycled = renewKeys(c.cycled, r)
	c.typeFuncs = renewKeys(c.typeFuncs, r)
	c.funcDepth = renewKeys(c.funcDepth, r)
	c.steps = renewKeys(c.steps, r)
	c.buffered = renewKeys(c.buffered, r)
	c.fnParams = renewKeys(c.fnParams, r)
	for _, ps := range c.fnParams { //canon:unordered each list is renewed in place
		renewSlice(ps, r)
	}
	c.keyedWrites = c.renewWrites(r)
	renewValues(c.typeObjects, r) // renewed by construction: its readers take only state, local mark and body, which a copy shares
	renewValues(c.fieldObjects, r)
	renewValues(c.memberChecks, r)
	for _, ms := range c.members { //canon:unordered each list is renewed in place
		renewSlice(ms, r)
	}
	for _, cs := range c.cases { //canon:unordered each list is renewed in place
		renewSlice(cs, r)
	}
}

// renewWrites moves each keyed list's key writes to its object checked again, when it is one,
// less the writes of `entry` declarations checked again, which write theirs again.
func (c *checker) renewWrites(r *renewer) map[*object]*keyWrites {
	out := make(map[*object]*keyWrites, len(c.keyedWrites))
	for table, w := range c.keyedWrites { //canon:unordered a map rebuilt
		kept := w.writes[:0:0]
		for _, kw := range w.writes {
			if !r.pl.redo[kw.entry] {
				kept = append(kept, keyWrite{entry: r.of(kw.entry), key: kw.key})
			}
		}
		out[r.of(table)] = &keyWrites{field: w.field, writes: kept}
	}
	return out
}

// renewBodies renews the fields and methods of every record, case and variant body.
func (c *checker) renewBodies(r *renewer) {
	seen := map[*recordCtx]bool{}
	visit := func(b *recordCtx) {
		if b == nil || seen[b] {
			return
		}
		seen[b] = true
		renewValues(b.fields, r)
		renewValues(b.methods, r)
		renewSlice(b.order, r)
	}
	for _, o := range c.typeObjects { //canon:unordered each body is renewed in place, once
		visit(o.body)
	}
	for _, cs := range c.cases { //canon:unordered each body is renewed in place, once
		for _, co := range cs {
			visit(co.body)
		}
	}
	for _, b := range c.caseBodies { //canon:unordered each body is renewed in place, once
		visit(b)
	}
	for _, b := range c.variantBody { //canon:unordered each body is renewed in place, once
		visit(b)
	}
}

// renewPackageState renews each package's names, declarations, entries, import scopes,
// duplicate keys and table keys.
func (c *checker) renewPackageState(r *renewer) {
	keys := map[*entryKeys]bool{}
	for _, p := range c.sorted {
		renewValues(p.names, r)
		renewSlice(p.all, r)
		for i, at := range p.entries {
			p.entries[i].obj = r.of(at.obj)
		}
		for _, fs := range p.scopes { //canon:unordered each scope is renewed in place
			renewValues(fs.names, r)
		}
		for i, d := range c.entryDups[p] {
			c.entryDups[p][i] = entryDup{entry: r.of(d.entry), table: r.of(d.table), first: r.of(d.first)}
		}
		for _, o := range p.all {
			if o.keys != nil && !keys[o.keys] {
				keys[o.keys] = true
				renewValues(o.keys.byName, r)
				renewSlice(o.keys.order, r)
			}
		}
	}
}

// renewSpans moves the spans recorded in a swapped file to its new version: a bound's file
// and, for a let's signature, its node; the keys written in a shared keyed list's literal,
// which the new version holds at the same offsets.
func (c *checker) renewSpans(pl *recheckPlan) {
	for b, at := range c.boundSpans { //canon:unordered each entry is renewed in place
		nf := pl.old[at.file]
		if nf == nil {
			continue
		}
		at.file = nf
		if n := pl.sig[at.node]; n != nil {
			at.node = n
		}
		c.boundSpans[b] = at
	}
	ids := map[source.FileID]source.FileID{}
	for old, nf := range pl.old { //canon:unordered a lookup table
		ids[old.Src.ID] = nf.Src.ID
	}
	// Renewed by construction: a shared list's key spans lie in the unchanged prefix, which the
	// append-only file set reads alike in the old file.
	for _, keys := range c.listKeys { //canon:unordered each entry is renewed in place
		for k, sp := range keys { //canon:unordered each entry is renewed in place
			if id, ok := ids[sp.File]; ok {
				sp.File = id
				keys[k] = sp
			}
		}
	}
}

// renew moves the journal's objects to the program checked again: the kept findings'
// origins, the breaks, the direct breaks, and the kept folds' owners and signature nodes.
func (j *journal) renew(r *renewer, pl *recheckPlan) {
	for i := range j.findings {
		f := &j.findings[i]
		f.from.decl, f.from.keyed = r.of(f.from.decl), r.of(f.from.keyed)
	}
	for i, o := range j.breaks {
		j.breaks[i] = r.obj(o)
	}
	j.direct = renewSet(j.direct, r.obj)
	for i := range j.folds {
		fc := &j.folds[i]
		fc.owner = r.obj(fc.owner)
		if n, ok := pl.sig[fc.e].(syntax.Expr); ok {
			fc.e = n
		}
	}
}

// renewKeys is m keyed by each object's renewal, less the objects checked again.
func renewKeys[V any](m map[*object]V, r *renewer) map[*object]V {
	out := make(map[*object]V, len(m))
	for k, v := range m { //canon:unordered a map rebuilt
		if !r.pl.redo[k] {
			out[r.of(k)] = v
		}
	}
	return out
}

// renewValues renews m's objects in place.
func renewValues[K comparable](m map[K]*object, r *renewer) {
	for k, o := range m { //canon:unordered each entry is renewed in place
		if n := r.of(o); n != o {
			m[k] = n
		}
	}
}

// renewSlice renews s's objects in place.
func renewSlice(s []*object, r *renewer) {
	for i, o := range s {
		s[i] = r.of(o)
	}
}

// renewSet is set keyed by each object's renewal.
func renewSet(set map[Object]bool, to func(Object) Object) map[Object]bool {
	out := make(map[Object]bool, len(set))
	for o, v := range set { //canon:unordered a set rebuilt
		out[to(o)] = v
	}
	return out
}
