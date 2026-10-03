package edit

import (
	"context"
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/value"
)

// visitFn sees a value, the container holding it (nil for a root's), and its path; true stops the walk.
type visitFn func(r rootRef, v, parent value.Value, segs []Seg) bool

// valueWalk visits the values of the lets, map keys included (through onKey when set), until
// visit returns true; with replaced set, also the values amendments replaced, each once. A let
// without a value is skipped, unless strict makes it the walk's error.
type valueWalk struct {
	visit    visitFn
	onKey    visitFn
	strict   func(rootRef, error) error
	roots    func(rootRef) bool
	replaced bool
	amended  map[check.Object]bool
	history  func(path ...value.Value) []value.Value // the analysis's, which a test counts
	seen     map[value.Value]bool
	visits   int
	done     bool
}

func (s *Snapshot) newWalk(visit visitFn, replaced bool) *valueWalk {
	return &valueWalk{
		visit: visit, roots: everyRoot, replaced: replaced, amended: s.amendedLets(),
		history: s.a.History, seen: map[value.Value]bool{},
	}
}

// amendedLets are the lets an amendment of any layer targets.
func (s *Snapshot) amendedLets() map[check.Object]bool {
	out := map[check.Object]bool{}
	for _, pkg := range s.pkgs {
		for _, blocks := range pkg.Layers { //canon:unordered builds a set
			for _, b := range blocks {
				out[s.info.NameUses[b.Target]] = true
			}
		}
	}
	return out
}

func everyRoot(rootRef) bool { return true }

// run walks the value of every let the walk's roots keep.
func (w *valueWalk) run(ctx context.Context, s *Snapshot) error {
	for _, pkg := range s.pkgs {
		for _, obj := range pkg.Decls {
			if err := ctx.Err(); err != nil {
				return err
			}
			if err := w.root(s, rootRef{pkg: pkg, obj: obj}); err != nil || w.done {
				return err
			}
		}
	}
	return nil
}

func (w *valueWalk) root(s *Snapshot, r rootRef) error {
	if r.obj.Kind() != check.ObjLet || !w.roots(r) {
		return nil
	}
	v, err := s.force(r)
	switch {
	case err == nil:
		w.value(r, v, nil, nil, false)
	case w.strict != nil:
		return w.strict(r, err)
	}
	return nil
}

// value visits v, its map keys, its children, then under an amended root what amendments
// replaced it with; an old value is visited once.
func (w *valueWalk) value(r rootRef, v, parent value.Value, segs []Seg, old bool) {
	if old && w.seen[v] {
		return
	}
	w.seen[v] = w.seen[v] || old
	w.visits++
	if w.done = w.visit(r, v, parent, segs); w.done {
		return
	}
	if w.keys(r, v, segs); w.done {
		return
	}
	for _, c := range children(v) {
		if w.value(r, c.v, v, append(slices.Clip(segs), c.seg), old); w.done {
			return
		}
	}
	if w.replaced && w.amended[r.obj] {
		w.olds(r, v, parent, segs)
	}
}

// keys visits a map's keys.
func (w *valueWalk) keys(r rootRef, v value.Value, segs []Seg) {
	m, ok := v.(*value.Map)
	if !ok {
		return
	}
	visit := w.visit
	if w.onKey != nil {
		visit = w.onKey
	}
	kt, _, _ := mapTypes(m.T)
	for _, k := range m.Keys {
		if w.done = visit(r, k, m, append(slices.Clip(segs), keySeg(k, kt))); w.done {
			return
		}
	}
}

// olds visits the values amendments replaced v with, at v's place.
func (w *valueWalk) olds(r rootRef, v, parent value.Value, segs []Seg) {
	hist := w.history(v)
	for i := 1; i < len(hist) && !w.done; i++ {
		w.value(r, hist[i], parent, segs, true)
	}
}
