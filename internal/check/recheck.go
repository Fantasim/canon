package check

import (
	"slices"

	"github.com/fantasim/canonlang/internal/syntax"
)

// recheckPlan is what one Recheck swaps: each changed file with the version it replaces.
type recheckPlan struct {
	swaps []swap
	old   map[*syntax.File]bool
	pkgs  map[string]bool // the swapped files' packages, by path
}

// swap is one changed file of a package: the checked version, its entries' objects in order,
// and the new version.
type swap struct {
	pkg      *pkgState
	old, new *syntax.File
	objs     []*object
}

// plan matches each changed file with the checked file of its path; it holds when each changed
// only in its bodies and no finding kept as it was reads one.
func (c *checker) plan(changed []*syntax.File) (*recheckPlan, bool) {
	pl := &recheckPlan{old: map[*syntax.File]bool{}, pkgs: map[string]bool{}}
	for _, nf := range changed {
		sw, ok := c.swapOf(nf)
		if !ok || pl.old[sw.old] {
			return nil, false
		}
		if sw.old == nf {
			continue
		}
		pl.swaps = append(pl.swaps, sw)
		pl.old[sw.old], pl.pkgs[sw.pkg.path] = true, true
	}
	return pl, c.keepable(pl)
}

// swapOf is the swap of a changed file, when its package holds a file of its path and the two
// differ only in their bodies.
func (c *checker) swapOf(nf *syntax.File) (swap, bool) {
	if nf == nil || nf.Package == nil {
		return swap{}, false
	}
	p := c.pkgs[qualified(nf.Package)]
	if p == nil {
		return swap{}, false
	}
	i := slices.IndexFunc(p.files, func(f *syntax.File) bool { return f.Src.Path == nf.Src.Path })
	if i < 0 || (p.files[i] != nf && !bodyOnly(p.files[i], nf)) {
		return swap{}, false
	}
	sw := swap{pkg: p, old: p.files[i], new: nf}
	for _, at := range p.entries {
		if at.file == sw.old {
			sw.objs = append(sw.objs, at.obj)
		}
	}
	return sw, true
}

// keepable reports that no finding or fold Recheck keeps as it was reads a file it swaps.
func (c *checker) keepable(pl *recheckPlan) bool {
	for _, f := range c.journal.findings {
		if pl.redoes(f.from) {
			continue
		}
		if slices.ContainsFunc(pl.swaps, func(sw swap) bool { return slices.Contains(f.reads, sw.old.Src.ID) }) {
			return false
		}
	}
	return !slices.ContainsFunc(c.journal.folds, func(fc foldCall) bool { return pl.dropped(fc.owner) })
}

// redoes reports a finding Recheck reports again from the new files, not as it was.
func (pl *recheckPlan) redoes(from origin) bool {
	switch {
	case from.decl != nil:
		return pl.old[from.decl.file]
	case from.file != nil:
		return pl.old[from.file]
	case from.keys != nil:
		return pl.pkgs[from.keys.path]
	case from.keyed != nil:
		return pl.pkgs[from.keyed.pkg]
	}
	return false
}

// dropped reports an object of a swapped file.
func (pl *recheckPlan) dropped(o Object) bool {
	return o != nil && pl.old[o.File()]
}

// bodyOnly reports two versions of a source file holding only `entry` declarations, alike but
// for their values and doc comments: package, imports, and each entry's modifiers, annotations,
// table and key. No type is written in either, so no written type, ref or bound changes.
func bodyOnly(old, nf *syntax.File) bool {
	if !entriesOnly(old) || !entriesOnly(nf) || len(old.Decls) != len(nf.Decls) || len(old.Imports) != len(nf.Imports) {
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
	for i, d := range old.Decls {
		if !sameSignature(old, nf, d.(*syntax.EntryDecl), nf.Decls[i].(*syntax.EntryDecl)) {
			return false
		}
	}
	return true
}

// entriesOnly reports a source file of `entry` declarations and no written type.
func entriesOnly(f *syntax.File) bool {
	if f.FileKind != syntax.FileSource || f.Layer != nil || f.Lang != nil || f.Project != nil || len(f.Amends)+len(f.Entries) > 0 {
		return false
	}
	for _, d := range f.Decls {
		if _, ok := d.(*syntax.EntryDecl); !ok {
			return false
		}
	}
	typed := false
	syntax.Inspect(f, func(n syntax.Node) bool {
		_, isType := n.(syntax.Type)
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
	if len(a.Annotations) != len(b.Annotations) {
		return false
	}
	for i, an := range a.Annotations {
		if writtenIn(of, an) != writtenIn(nf, b.Annotations[i]) {
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
