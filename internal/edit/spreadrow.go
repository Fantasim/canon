package edit

import (
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
)

// spreadRow is a base Set, Reset or Remove of a field a spread supplies, or a base Set or Reset of
// a whole record beside a spread, when an active layer amends such a field on the spread's source or below
// it: the write and its E23 inverse make it explicit, hiding the layer's value (API.md E22, W9).
func spreadRow(j *judge) Reason {
	if j.layer != "" || len(j.s.a.Layers()) == 0 {
		return ReasonNone
	}
	layer, hit := j.spreadFieldLayer()
	if !hit {
		layer, hit = j.spreadRecordLayer()
	}
	if !hit {
		return ReasonNone
	}
	j.blocker = layer
	return ReasonLayered
}

// spreadFieldLayer is the layer amending the field a spread supplies at the path, for a Set, Reset
// or Remove of it.
func (j *judge) spreadFieldLayer() (string, bool) {
	n := len(j.res.Steps)
	if n == 0 || j.last().state != stSpread || j.op != OpSet && j.op != OpReset && j.op != OpRemove {
		return "", false
	}
	return j.s.spreadsAmend(braceOf(j.cur[n-1].node), []Seg{j.res.Steps[n-1].Seg})
}

// spreadRecordLayer is the layer amending a field a spread supplies to the record at the path, for
// a Set or Reset of that whole record, its literal beside the spread.
func (j *judge) spreadRecordLayer() (string, bool) {
	c := j.last()
	rec, ok := j.res.Target.(*value.Record)
	if j.op != OpSet && j.op != OpReset || !ok || c.state != stTree || c.mode != ModeCanon || !hasSpread(c.node) {
		return "", false
	}
	var fields []Seg
	for _, f := range fieldsOf(rec.T) {
		if f.Input == nil && fieldItem(c.node, f.Name) == nil {
			fields = append(fields, Seg{Kind: SegField, Name: f.Name})
		}
	}
	return j.s.spreadsAmend(braceOf(c.node), fields)
}

// spreadsAmend is the active layer amending one of fields on the source of a spread of lit.
func (s *Snapshot) spreadsAmend(lit *syntax.BraceLit, fields []Seg) (string, bool) {
	if lit == nil {
		return "", false
	}
	for _, it := range lit.Items {
		sp, ok := it.(*syntax.SpreadItem)
		if !ok {
			continue
		}
		for _, f := range fields {
			if layer, amended := s.amendsBelow(sp.X, f); amended {
				return layer, true
			}
		}
	}
	return "", false
}

// amendsBelow is the active layer whose line can change field of the value spread expression e
// names, following that field through every spread and let the base reads it from (spreadLayer).
func (s *Snapshot) amendsBelow(e syntax.Expr, field Seg) (string, bool) {
	r, segs, ok := s.spreadSource(syntax.Unparen(e))
	if !ok {
		return "", false
	}
	return s.spreadLayer(r, append(segs, field), map[string]bool{})
}

// spreadLayer is the active layer with a line on, above or below field path segs of root r, or on
// a value the base reads it through: a spread supplying it, a let naming it; seen guards cycles.
func (s *Snapshot) spreadLayer(r rootRef, segs []Seg, seen map[string]bool) (string, bool) {
	src := Path{Package: r.pkg.Path, Root: r.obj.Name(), Segs: segs}.String()
	if seen[src] || len(seen) >= maxSpreadHops {
		return "", false
	}
	seen[src] = true
	if x, ok := s.layerOn(r, src); ok {
		return x, true
	}
	return s.readThrough(syntax.Unparen(initializer(r.obj.Decl())), segs, seen)
}

// layerOn is the first active layer with a line on root r on, above or below path src.
func (s *Snapshot) layerOn(r rootRef, src string) (string, bool) {
	for _, x := range s.a.Layers() {
		for _, f := range layerFiles(r.pkg, x) {
			if slices.ContainsFunc(f.Amends, func(b *syntax.AmendBlock) bool { return s.blockOn(f, b, r, src) }) {
				return x, true
			}
		}
	}
	return "", false
}

// blockOn reports block b of f amending root r with a line on, above or below src.
func (s *Snapshot) blockOn(f *syntax.File, b *syntax.AmendBlock, r rootRef, src string) bool {
	if b.Target == nil || b.Target.Name != r.obj.Name() {
		return false
	}
	return slices.ContainsFunc(b.Items, func(it *syntax.Amendment) bool { return overlaps(s.linePathOf(f, r.at(), it.Path), src) })
}

// readThrough is spreadLayer of the value field path segs reads in expression e: a let it names,
// the field's item in a literal, or each spread of a literal leaving the field out.
func (s *Snapshot) readThrough(e syntax.Expr, segs []Seg, seen map[string]bool) (string, bool) {
	for i, sg := range segs {
		if r, rs, ok := s.spreadSource(e); ok {
			return s.spreadLayer(r, append(slices.Clip(rs), segs[i:]...), seen)
		}
		if fi := fieldItem(e, sg.Name); fi != nil {
			e = syntax.Unparen(fi.Value)
			continue
		}
		return s.spreadsOf(e, segs[i:], seen)
	}
	if r, rs, ok := s.spreadSource(e); ok {
		return s.spreadLayer(r, rs, seen)
	}
	return "", false
}

// spreadsOf is spreadLayer of segs through each spread of literal e, which leaves its first field out.
func (s *Snapshot) spreadsOf(e syntax.Expr, segs []Seg, seen map[string]bool) (string, bool) {
	lit := braceOf(e)
	if lit == nil {
		return "", false
	}
	for _, it := range lit.Items {
		sp, ok := it.(*syntax.SpreadItem)
		if !ok {
			continue
		}
		if r, rs, ok := s.spreadSource(syntax.Unparen(sp.X)); ok {
			if x, found := s.spreadLayer(r, append(slices.Clip(rs), segs...), seen); found {
				return x, true
			}
		}
	}
	return "", false
}

// spreadSource is the root and field path an expression names: a let, possibly qualified by its
// package, then fields; false for any other expression.
func (s *Snapshot) spreadSource(e syntax.Expr) (rootRef, []Seg, bool) {
	switch x := e.(type) {
	case *syntax.IdentExpr:
		return s.letRoot(s.info.Uses[x])
	case *syntax.SelectorExpr:
		if r, segs, ok := s.letRoot(s.info.NameUses[x.Name]); ok {
			return r, segs, true // a let qualified by its package
		}
		if x.X == nil {
			return rootRef{}, nil, false
		}
		r, segs, ok := s.spreadSource(syntax.Unparen(x.X))
		return r, append(segs, Seg{Kind: SegField, Name: x.Name.Name}), ok
	}
	return rootRef{}, nil, false
}

// letRoot is the root of obj when it is a let.
func (s *Snapshot) letRoot(obj check.Object) (rootRef, []Seg, bool) {
	if obj == nil || obj.Kind() != check.ObjLet {
		return rootRef{}, nil, false
	}
	r, err := s.lookup(Path{Package: obj.Pkg(), Root: obj.Name()})
	return r, nil, err == nil && r.enum == nil
}
