package check

import (
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
)

// recheckPlan is what one Recheck swaps: each changed file with the version it replaces.
type recheckPlan struct {
	swaps []swap
	old   map[*syntax.File]*syntax.File // each replaced file, to its new version
	pkgs  map[string]bool               // the swapped files' packages, by path
	redo  map[*object]bool              // the objects of the declarations checked again: entries, lets, their rows
	sig   map[syntax.Node]syntax.Node   // each node of a let checked again outside its value, to its new one
	wide  bool                          // a swap shares declarations or checks a let again: every reference is renewed
}

// swap is one changed file of a package: the checked version, the new one, and their
// declarations in pairs.
type swap struct {
	pkg      *pkgState
	old, new *syntax.File
	pairs    []declPair
	shared   map[syntax.Node]bool // the nodes both versions hold: header parts and declarations
}

// declPair is a declaration of the checked file and its place in the new one: the same node
// (shared, NFR-02), or an `entry` or annotated `let` checked again, its old object and, for a
// table literal, the objects of its rows.
type declPair struct {
	old, new syntax.Decl
	obj      *object
	rows     []*object
}

// shared reports a declaration both versions hold.
func (d declPair) shared() bool { return d.old == d.new }

// plan matches each changed file with the checked file of its path; it holds when each keeps
// its declarations' signatures and no finding or fold kept as it was reads one.
func (c *checker) plan(changed []*syntax.File, bags Bags) (*recheckPlan, bool) {
	pl := &recheckPlan{old: map[*syntax.File]*syntax.File{}, pkgs: map[string]bool{}, redo: map[*object]bool{}, sig: map[syntax.Node]syntax.Node{}}
	for _, nf := range changed {
		sw, ok := c.swapOf(nf, bags)
		if !ok || pl.old[sw.old] != nil {
			return nil, false
		}
		if sw.old == nf {
			continue
		}
		if !pl.add(sw) {
			return nil, false
		}
	}
	return pl, c.keepable(pl)
}

// add puts sw in the plan: its declarations checked again, their signatures' nodes paired.
func (pl *recheckPlan) add(sw swap) bool {
	pl.swaps = append(pl.swaps, sw)
	pl.old[sw.old], pl.pkgs[sw.pkg.path] = sw.new, true
	for _, d := range sw.pairs {
		if d.shared() {
			pl.wide = true
			continue
		}
		pl.redo[d.obj] = true
		for _, r := range d.rows {
			pl.redo[r] = true
		}
		if l, ok := d.old.(*syntax.LetDecl); ok {
			pl.wide = true
			if !pairSignature(pl.sig, l, d.new.(*syntax.LetDecl)) {
				return false
			}
		}
	}
	return true
}

// swapOf is the swap of a changed file, when its package holds a file of its path and the
// two keep every declaration's signature.
func (c *checker) swapOf(nf *syntax.File, bags Bags) (swap, bool) {
	if nf == nil || nf.Package == nil {
		return swap{}, false
	}
	p := c.pkgs[qualified(nf.Package)]
	if p == nil {
		return swap{}, false
	}
	i := slices.IndexFunc(p.files, func(f *syntax.File) bool { return f.Src.Path == nf.Src.Path })
	if i < 0 {
		return swap{}, false
	}
	sw := swap{pkg: p, old: p.files[i], new: nf}
	if sw.old == nf {
		return sw, true
	}
	if !sameHeader(sw.old, nf) {
		return swap{}, false
	}
	return c.pairDecls(sw, bags[p.path])
}

// keepable reports that no finding or fold Recheck keeps as it was reads a file it swaps, and
// that no fold of a declaration checked again is kept but a let's signature's.
func (c *checker) keepable(pl *recheckPlan) bool {
	for _, f := range c.journal.findings {
		if pl.redoes(f) {
			continue
		}
		if slices.ContainsFunc(pl.swaps, func(sw swap) bool { return slices.Contains(f.reads, sw.old.Src.ID) }) {
			return false
		}
	}
	return !slices.ContainsFunc(c.journal.folds, func(fc foldCall) bool { return pl.dropped(fc.owner) && pl.sig[fc.e] == nil })
}

// redoes reports a finding Recheck reports again from the new files, not as it was: a
// declaration checked again reports again what its check reported, not what step 1 or 2 did.
func (pl *recheckPlan) redoes(f journaled) bool {
	from := f.from
	switch {
	case from.decl != nil:
		return pl.redo[from.decl] && f.body
	case from.file != nil:
		return pl.old[from.file] != nil
	case from.keys != nil:
		return pl.pkgs[from.keys.path]
	case from.keyed != nil:
		return pl.pkgs[from.keyed.pkg]
	}
	return false
}

// dropped reports an object of a declaration checked again.
func (pl *recheckPlan) dropped(o Object) bool {
	x, ok := o.(*object)
	return ok && pl.redo[x]
}

// sameHeader reports two versions of a source file of declarations alone, with the same
// package clause and imports as written.
func sameHeader(old, nf *syntax.File) bool {
	if !declsOnly(old) || !declsOnly(nf) || len(old.Decls) != len(nf.Decls) || len(old.Imports) != len(nf.Imports) {
		return false
	}
	if writtenIn(old, old.Package) != writtenIn(nf, nf.Package) {
		return false
	}
	for i, imp := range old.Imports {
		if writtenIn(old, imp) != writtenIn(nf, nf.Imports[i]) {
			return false
		}
	}
	return true
}

// declsOnly reports a source file of top-level declarations: no layer, translation or project.
func declsOnly(f *syntax.File) bool {
	return f.FileKind == syntax.FileSource && f.Layer == nil && f.Lang == nil && f.Project == nil && len(f.Amends)+len(f.Entries) == 0
}

// typeless reports a node holding no written type.
func typeless(n syntax.Node) bool {
	typed := false
	syntax.Inspect(n, func(x syntax.Node) bool {
		_, isType := x.(syntax.Type)
		typed = typed || isType
		return !typed
	})
	return !typed
}

// sameSignature reports two `entry` declarations alike but for their value and doc comment.
func sameSignature(of, nf *syntax.File, a, b *syntax.EntryDecl) bool {
	if writtenIn(of, a.Table) != writtenIn(nf, b.Table) || writtenIn(of, a.Key) != writtenIn(nf, b.Key) || writtenIn(of, a.Mods) != writtenIn(nf, b.Mods) {
		return false
	}
	return sameAnnotations(of, nf, a.Annotations, b.Annotations)
}

// sameAnnotations reports two annotation lists written alike.
func sameAnnotations(of, nf *syntax.File, a, b []*syntax.Annotation) bool {
	if len(a) != len(b) {
		return false
	}
	for i, an := range a {
		if writtenIn(of, an) != writtenIn(nf, b[i]) {
			return false
		}
	}
	return true
}

// writtenIn is a node's source text in f, "" for none.
func writtenIn(f *syntax.File, n syntax.Node) string {
	sp := f.Span(n)
	return string(f.Src.Content[sp.Start:sp.End])
}

// parseClean reports f holding no parse finding in bag, which a truncated bag cannot tell.
func parseClean(bag *diag.Bag, f *syntax.File) bool {
	if bag == nil {
		return true
	}
	if len(bag.Summary().Truncated) > 0 {
		return false
	}
	for _, x := range bag.Findings() {
		if x.Span.File == f.Src.ID && syntaxCode(x.Code) {
			return false
		}
	}
	return true
}
