package eval

import (
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
)

// Outermost is stack and more with f, if any, outermost; at the cap or past a cut it replaces the
// outermost kept frame, cut the more unless hidden, f among the frames cut (DECISIONS 324).
func Outermost(stack []diag.Frame, more int, f *diag.Frame, hidden bool) ([]diag.Frame, int) {
	switch {
	case f == nil || slices.Contains(stack, *f):
		return stack, more
	case len(stack) == 0 && hidden:
		return []diag.Frame{*f}, more - 1 // revealed from the cut
	case more == 0 && len(stack) < diag.MaxStackFrames, len(stack) == 0:
		return append(slices.Clone(stack), *f), more
	}
	out := slices.Clone(stack)
	out[len(out)-1] = *f
	if hidden {
		return out, more // one frame hidden, one revealed
	}
	return out, more + 1
}

// Hides reports f among the frames cut from stack, a provenance stack this evaluator or its parent cut.
func (e *Evaluator) Hides(stack []diag.Frame, f *diag.Frame) bool {
	if f == nil || len(stack) == 0 {
		return false
	}
	for x := e; x != nil; x = x.parent {
		if top, ok := x.cuts[&stack[0]]; ok {
			return top == *f
		}
	}
	return false
}

// noteCut keeps, in a precomputation, the outermost frame of f's cut stack, once per frame that caches it.
func (r *run) noteCut(f *frame) {
	if !r.notesCuts || f.more == 0 || len(f.stack) == 0 || f.top == nil {
		return
	}
	e := r.ev
	if e.cuts == nil {
		e.cuts = map[*diag.Frame]diag.Frame{}
	}
	e.cuts[&f.stack[0]] = diag.Frame{Fn: f.top.fn, Span: f.top.call}
}

// ForgetCuts drops what the last precomputation noted of its cut stacks, once its result is judged.
func (e *Evaluator) ForgetCuts() {
	e.cuts = nil
}
