package edit

import (
	"context"
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
)

// found is a value or key reference, the path it was reached by, and whether code computed it.
type found struct {
	ref      Ref
	root     rootRef
	segs     []Seg
	computed bool
}

// refScan is one Refs call's walk of the lets, active and replaced values alike: the name
// tokens of each file read so far, and the references found.
type refScan struct {
	s      *Snapshot
	tg     *target
	w      *valueWalk
	tokens map[source.FileID]map[source.Span]bool
	found  []found
}

// valueRefs are the target's values and map keys in every let, active and replaced by
// amendments alike, one per span (log-2026-09-29 M4 U4a); covers are the spans stating them,
// whose names are no other reference, sorted.
func (s *Snapshot) valueRefs(ctx context.Context, tg *target) ([]Ref, []source.Span, error) {
	sc := s.newScan(tg)
	if err := sc.lets(ctx); err != nil {
		return nil, nil, err
	}
	var refs []Ref
	var covers []source.Span
	for _, f := range s.firstPaths(sc.found) {
		refs = append(refs, f.ref)
		if !f.computed {
			covers = append(covers, f.ref.Span)
		}
	}
	slices.SortFunc(covers, spanOrder)
	return refs, covers, nil
}

func (s *Snapshot) newScan(tg *target) *refScan {
	sc := &refScan{s: s, tg: tg, tokens: map[source.FileID]map[source.Span]bool{}}
	sc.w = s.newWalk(sc.noted(RefValue), true)
	sc.w.onKey = sc.noted(RefKey)
	sc.w.strict = tg.missing // a let that could hold the target but has no value fails the list
	return sc
}

// lets walks the value of every let of every package.
func (sc *refScan) lets(ctx context.Context) error {
	return sc.w.run(ctx, sc.s)
}

// noted is the walk's visit: a value or map key that is the target is noted as kind.
func (sc *refScan) noted(kind RefKind) visitFn {
	return func(r rootRef, v, _ value.Value, segs []Seg) bool {
		if sc.tg.isValue(v) {
			sc.note(r, kind, v, segs)
		}
		return false
	}
}

// note keeps v when a source states it or code computes it (log-2026-09-29 M4 U4a); a
// default, a spread or a layer states it in code, where its name is the reference.
func (sc *refScan) note(root rootRef, kind RefKind, v value.Value, segs []Seg) {
	p := v.Prov()
	if p == nil {
		return
	}
	keep, computed := sc.stated(p)
	if !keep {
		return
	}
	path := Path{Root: root.obj.Name(), Segs: segs}
	ref := Ref{Kind: kind, Package: root.pkg.Path, Path: path.String(), Span: p.Span}
	if computed {
		ref.Span = nameSpan(root, p.Span) // where the reference is written (log-2026-09-29 M4 U4a round 3)
	}
	sc.found = append(sc.found, found{ref: ref, root: root, segs: segs, computed: computed})
}

// nameSpan is the span of the name of the let root declares, orElse when it has none.
func nameSpan(root rootRef, orElse source.Span) source.Span {
	d, ok := root.obj.Decl().(*syntax.LetDecl)
	if f := root.obj.File(); ok && f != nil && d.Name != nil {
		return f.Span(d.Name)
	}
	return orElse
}

// foundKey groups the paths of one reference: its span, and for a computed one its let, which
// holds a value of its own.
type foundKey struct {
	span source.Span
	let  check.Object
}

// firstPaths keeps one reference per key: the first whose path is structural, else the first.
func (s *Snapshot) firstPaths(all []found) []found {
	byKey := map[foundKey][]found{}
	var order []foundKey
	for _, f := range all {
		k := foundKey{span: f.ref.Span}
		if f.computed {
			k.let = f.root.obj
		}
		if byKey[k] == nil {
			order = append(order, k)
		}
		byKey[k] = append(byKey[k], f)
	}
	out := make([]found, 0, len(order))
	for _, k := range order {
		group := byKey[k]
		pick := group[0]
		if len(group) > 1 {
			if i := slices.IndexFunc(group, s.reachesSource); i >= 0 {
				pick = group[i]
			}
		}
		out = append(out, pick)
	}
	return out
}

// reachesSource reports a path that stays in its root's source tree (API.md W3).
func (s *Snapshot) reachesSource(f found) bool {
	_, ok := s.structural(Path{Package: f.root.pkg.Path, Root: f.root.obj.Name(), Segs: f.segs})
	return ok
}
