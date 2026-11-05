package edit

import (
	"errors"
	"slices"

	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
)

// shiftRow is a base Move in a collection an active layer's line addresses by position: `[#n]`,
// or `[i]` in a plain list. The Move changes which item that position names, and the Move back
// would meet the amendment (API.md E22, W10); lines, not values, so Editable and Apply agree.
func shiftRow(j *judge) Reason {
	n := len(j.res.Steps)
	if j.layer != "" || j.op != OpMove || n == 0 {
		return ReasonNone
	}
	holder := j.res
	holder.Resolved = Resolved{Canonical: j.res.Canonical, Steps: j.res.Steps[:n-1], Target: j.res.parent(n - 1)}
	plain := !keyedContainer(holder.Target.Type())
	if _, isMap := holder.Target.(*value.Map); isMap {
		plain = false
	}
	for _, x := range j.s.a.Layers() {
		if j.s.lineBelow(holder, x, func(_ *syntax.File, it *syntax.Amendment) bool { return positional(it.Path[n-1], plain) }) {
			j.blocker = x
			return ReasonLayered
		}
	}
	return ReasonNone
}

// positional reports an amend segment naming an item by its position: `[#n]`, or an integer `[i]`
// in a plain list, where it is no key (EVALUATION.md 9.2).
func positional(seg *syntax.AmendSegment, plain bool) bool {
	if seg.Position != nil {
		return true
	}
	if seg.Key == nil {
		return false
	}
	_, isInt := syntax.Unparen(seg.Key).(*syntax.IntLit)
	return plain && isInt
}

// layerBlockable reports an inverse of undo an active layer's line can block: one names the
// inverse's holder, or a value on, above or below it.
func (a *applier) layerBlockable(undo []Operation) bool {
	return slices.ContainsFunc(undo, func(op Operation) bool { return op.Kind != OpRenameName && a.session.snap.layerNear(op.Path) })
}

// layerNear reports a line of an active layer amending path's holder, or a value on, above or
// below it.
func (s *Snapshot) layerNear(path string) bool {
	p, err := Parse(path)
	if err != nil {
		return false
	}
	r, err := s.lookup(p)
	if err != nil || r.enum != nil {
		return false
	}
	holder, ok := enclosing(path)
	if !ok {
		holder = path
	}
	for _, x := range s.a.Layers() {
		for _, f := range layerFiles(r.pkg, x) {
			if slices.ContainsFunc(f.Amends, func(b *syntax.AmendBlock) bool { return s.blockNear(f, b, r, holder) }) {
				return true
			}
		}
	}
	return false
}

// blockNear reports block b of f amending root r with a line on, above or below holder.
func (s *Snapshot) blockNear(f *syntax.File, b *syntax.AmendBlock, r rootRef, holder string) bool {
	if b.Target == nil || b.Target.Name != r.obj.Name() {
		return false
	}
	return slices.ContainsFunc(b.Items, func(it *syntax.Amendment) bool { return overlaps(s.linePathOf(f, r.at(), it.Path), holder) })
}

// overlaps reports two canonical paths one of which is the other or below it.
func overlaps(p, q string) bool {
	return within(p, q, pathMarks, true) || within(q, p, pathMarks, true)
}

// undoLine is err, an inverse's refusal at path, named by the operation that path falls on; a
// failure no inverse causes stays as it is (fatal).
func undoLine(path string, err error) error {
	if fatal(err) {
		return err
	}
	var ne *NotEditableError
	if errors.As(err, &ne) {
		err = ne
	}
	return &undoLineError{line: path, err: err}
}
