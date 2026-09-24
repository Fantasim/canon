package progen_test

import (
	"bytes"
	"path"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/testkit/progen"
)

// stillPlaced tells a shrunk candidate q of a "missing" failure that its operator would still
// make: undone, q holds a site of the operator that, applied, gives q back. Else shrinking cut
// what the rule needs, and the missing finding proves nothing.
func stillPlaced(f failure, q *progen.Project, written []progen.Place) bool {
	u := undo(q, written, f.m.Was)
	for _, tg := range targetsOf(u, f.run.pkgs) {
		for _, s := range f.o.sites(tg) {
			if s.Path == "" {
				s.Path = tg.path
			}
			m, err := s.Mutate(u)
			if err == nil && sameFiles(m.Project, q) {
				return true
			}
		}
	}
	return false
}

// undo is q with every written region given back what it held, from the last to the first so
// that the earlier regions do not move.
func undo(q *progen.Project, written []progen.Place, was []progen.Undo) *progen.Project {
	u := q.Clone()
	order := make([]int, len(written))
	for i := range order {
		order[i] = i
	}
	slices.SortFunc(order, func(a, b int) int {
		if d := written[b].Start - written[a].Start; d != 0 {
			return d
		}
		return written[b].End - written[a].End
	})
	for _, i := range order {
		w := written[i]
		src, _ := u.Get(w.Path)
		switch {
		case was[i].Absent:
			u.Remove(w.Path)
		case w.End <= len(src):
			u.Set(w.Path, slices.Concat(src[:w.Start], was[i].Text, src[w.End:]))
		}
	}
	return u
}

// targetsOf are the targets of a shrunk project, each file in its corpus package, else the one
// its directory names; project.canon in the first selected package.
func targetsOf(p *progen.Project, pkgs []string) []target {
	var out []target
	for _, name := range p.Names() {
		src, _ := p.Get(name)
		tg := target{pkg: packageOf(name), path: name, src: src}
		switch {
		case name == projectFile && len(pkgs) > 0:
			tg.pkg, tg.file = pkgs[0], parse(name, src)
		case path.Ext(name) == ".canon":
			tg.file = parse(name, src)
		}
		out = append(out, tg)
	}
	for i := range out {
		out[i].all = &out
	}
	return out
}

// packageOf is the corpus package of a file, else the one its directory names.
func packageOf(name string) string {
	if corpusVal != nil {
		if i := slices.IndexFunc(corpusVal.targets, func(tg target) bool { return tg.path == name }); i >= 0 {
			return corpusVal.targets[i].pkg
		}
	}
	return strings.ReplaceAll(path.Dir(name), "/", ".")
}

func sameFiles(a, b *progen.Project) bool {
	names := a.Names()
	if !slices.Equal(names, b.Names()) {
		return false
	}
	for _, n := range names {
		x, _ := a.Get(n)
		y, _ := b.Get(n)
		if !bytes.Equal(x, y) {
			return false
		}
	}
	return true
}
