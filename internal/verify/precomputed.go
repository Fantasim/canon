package verify

import (
	"context"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// result is a precomputed result's walk: its frame, and what the walk marked invalid.
type result struct {
	frame diag.Frame
	own   map[value.Value]bool
}

// CheckAs verifies val, a precomputed result, against t under frame, charged to root (DECISIONS 324).
func (v *Verifier) CheckAs(ctx context.Context, root eval.Root, val value.Value, t types.Type, frame diag.Frame) (Result, error) {
	return v.check(ctx, root, val, t, &result{frame: frame, own: map[value.Value]bool{}})
}

// Hider tells whether a frame is among those a provenance stack cut: the evaluator does.
type Hider interface {
	Hides(stack []diag.Frame, f *diag.Frame) bool
}

// ReportUnder reports b at s with no value path, f outermost in its stack; h, if any, tells a cut f (DECISIONS 324).
func (s Site) ReportUnder(b *diag.Builder, f diag.Frame, bag *diag.Bag, h Hider) {
	var stack []diag.Frame
	more := 0
	if p := s.prov; p != nil {
		b.Pointer(p.Pointer).Layer(p.Layer)
		stack, more = p.Stack, p.MoreFrames
	}
	stack, more = eval.Outermost(stack, more, &f, h != nil && h.Hides(stack, &f))
	b.Path("").Stack(stack).MoreFrames(more).Report(bag)
}

// reportAt reports b at s and path into the walk's bag; a precomputed result's with no path, under its frame.
func (w *walker) reportAt(s Site, b *diag.Builder, path string) {
	if w.result != nil {
		h, _ := w.ev.(Hider)
		s.ReportUnder(b, w.result.frame, w.bag, h)
		return
	}
	s.reportAt(b, path, w.bag)
}

// rootPath is where the walk starts: the top-level value's name, none for a precomputed result.
func (w *walker) rootPath() *Path {
	if w.result != nil {
		return Root("")
	}
	return Root(w.root.Name)
}

// foundElsewhere reports a part another walk found invalid, making the result invalid (EVALUATION.md §14).
func (w *walker) foundElsewhere(v value.Value) bool {
	if w.result == nil || w.result.own[v] {
		return false
	}
	w.voidRec() // a mark is read
	if !w.ev.Invalid(v) {
		return false
	}
	w.res.Valid = false
	for _, r := range w.recording {
		r.unfound = true
	}
	return true
}

// reportsAgain reports whether a finding remembered into bag is reported anew here (EVALUATION.md §14).
func (w *walker) reportsAgain(bag *diag.Bag) bool {
	return bag != w.bag && w.result == nil
}

// evalPath is the path the evaluator reports at: none in a precomputed result (EVALUATION.md §2.3).
func (w *walker) evalPath(at *Path) string {
	if w.result != nil {
		return ""
	}
	return at.String()
}

// predicate re-runs a where predicate on v, a precomputed result's under its frame.
func (w *walker) predicate(p *types.Predicate, v value.Value, at *Path) (bool, bool) {
	if w.result != nil && w.stage != nil {
		return w.stage.Where(w.ctx, p, v)
	}
	return w.ev.Where(w.ctx, p, v, w.evalPath(at))
}

// unbound is a level-1 ref left unbound: kept for the caller's E3505, reported at once in a precomputed result.
func (w *walker) unbound(r *value.Ref, at *Path) {
	if w.result == nil || w.stage == nil {
		w.res.Unbound = append(w.res.Unbound, Unbound{Ref: r, Path: w.evalPath(at)})
		w.invalid(r)
		return
	}
	w.stage.Unbound(r)
	w.invalid(r)
	for _, rec := range w.recording {
		rec.unfound = true
	}
}
