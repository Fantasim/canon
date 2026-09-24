package eval

import (
	"context"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
)

// run is one root being evaluated (EVALUATION.md §7.1).
type run struct {
	ev        *Evaluator
	ctx       context.Context
	charge    charge
	fr        *frame
	depth     int
	failed    bool
	tainted   bool
	free      bool      // stage B's where re-runs cost nothing (DECISIONS 148)
	sink      *diag.Bag // captures the findings of an expect subject
	site      source.Span
	chainNone bool
	at        *vpath
	coll      *collHint
	reports   *[]CheckReport
	poisonAt  string
	h         *stdHost
	test      *testState
	freeSteps int64
}

// frame is one call frame, or a root's own frame (fn empty).
type frame struct {
	vars   map[check.Object]value.Value
	self   value.Value
	it     value.Value
	short  value.Value
	file   *syntax.File
	pkg    string
	fn     string
	call   source.Span
	caller *frame
	ret    value.Value
	stack  []diag.Frame
	more   int
	cached bool
}

func (e *Evaluator) newRun(ctx context.Context, c charge, file *syntax.File) *run {
	return &run{ev: e, ctx: ctx, charge: c, fr: e.rootFrame(file)}
}

// rootFrame is the frame of a root evaluated in file.
func (e *Evaluator) rootFrame(file *syntax.File) *frame {
	return &frame{vars: map[check.Object]value.Value{}, file: file, pkg: e.index.pkg[file]}
}

// step spends one step for node n; the last one is E4401 (EVALUATION.md §12.2).
func (r *run) step(n syntax.Node) bool {
	return r.spend(1, func() source.Span { return r.span(n) })
}

func (r *run) spend(n int, at func() source.Span) bool {
	e := r.ev
	switch {
	case r.failed:
		return false
	case e.exhausted:
		r.failed = true
		return false
	case r.free:
		return r.spendFree(n, at)
	case n == 0:
		return true
	}
	if e.spent[r.charge] == 0 {
		e.order = append(e.order, r.charge)
	}
	before := e.steps
	e.steps += int64(n)
	e.spent[r.charge] += int64(n)
	if e.steps >= e.budget {
		r.budgetOut(at())
		return false
	}
	if before/cancelEvery != e.steps/cancelEvery && r.ctx.Err() != nil {
		e.exhausted, r.failed = true, true
		return false
	}
	return true
}

// spendFree counts the steps of a run that costs nothing (DECISIONS 148): it still stops on
// cancellation, and at a count the size of the budget, so it always ends (DECISIONS 188).
func (r *run) spendFree(n int, at func() source.Span) bool {
	before := r.freeSteps
	r.freeSteps += int64(n)
	if r.freeSteps >= r.ev.budget {
		r.budgetOut(at())
		return false
	}
	if before/cancelEvery != r.freeSteps/cancelEvery && r.ctx.Err() != nil {
		r.ev.exhausted, r.failed = true, true
		return false
	}
	return true
}

// budgetOut is E4401 with the heaviest charge; evaluation stops (EVALUATION.md §12.2).
func (r *run) budgetOut(at source.Span) {
	e := r.ev
	e.exhausted, r.failed = true, true
	var heavy charge
	for _, c := range e.order {
		if e.spent[c] > e.spent[heavy] {
			heavy = c
		}
	}
	b := diag.E4401.At(at, e.budget, r.qualified(heavy.pkg, heavy.name), e.spent[heavy])
	if bag := e.bagOf(r.fr.pkg); bag != nil {
		b.Stack(r.frames()).Report(bag)
	}
}

// fail reports a hard error and aborts the root; a tainted root aborts silently (§7.3).
func (r *run) fail(b *diag.Builder) {
	if r.failed {
		return
	}
	r.failed = true
	if !r.tainted || r.sink != nil {
		r.emit(b.Stack(r.frames()))
	}
}

// stop aborts the root without a finding: a poisoned read (EVALUATION.md §7.2).
func (r *run) stop() {
	r.failed = true
}

// emit reports a finding into the capture of an expect, else its package's bag (a fold's
// one bag while folding).
func (r *run) emit(b *diag.Builder) {
	if r.sink != nil {
		b.Report(r.sink)
		return
	}
	if bag := r.ev.bagOf(r.fr.pkg); bag != nil {
		b.Report(bag)
	}
}

// soft reports a soft finding about v and marks it invalid (EVALUATION.md §4.3, §7.1).
func (r *run) soft(b *diag.Builder, v value.Value, at *vpath) {
	if p := origin(v.Prov()); p != nil {
		b.Pointer(p.Pointer).Layer(p.Layer).Stack(p.Stack).MoreFrames(p.MoreFrames)
	}
	r.emit(b.Path(at.String()))
	r.ev.MarkInvalid(v)
}

// origin is where findings about a value are located: through spreads to the copied value (§13).
func origin(p *value.Prov) *value.Prov {
	for p != nil && p.Kind == value.ProvSpread && p.Via != nil {
		p = p.Via
	}
	return p
}

// located is the span findings about v take: its origin, else fallback.
func located(v value.Value, fallback source.Span) source.Span {
	if p := origin(v.Prov()); p != nil {
		return p.Span
	}
	return fallback
}

// read notes a value read; an invalid one taints the root (EVALUATION.md §7.3).
func (r *run) read(v value.Value) value.Value {
	if v != nil && r.ev.invalid[v] {
		r.tainted = true
	}
	return v
}

// span locates n in the current file.
func (r *run) span(n syntax.Node) source.Span {
	if r.fr.file == nil || n == nil {
		return source.Span{}
	}
	return r.fr.file.Span(n)
}

// qualified names a declaration of pkg as the current package sees it (ERRORS.md §1.3 Name).
func (r *run) qualified(pkg, name string) string {
	if pkg == "" || pkg == r.fr.pkg {
		return name
	}
	return pkg + dot + name
}

// frames is the Canon call stack, innermost first (EVALUATION.md §13).
func (r *run) frames() []diag.Frame {
	var out []diag.Frame
	for f := r.fr; f != nil; f = f.caller {
		if f.fn != "" {
			out = append(out, diag.Frame{Fn: f.fn, Span: f.call})
		}
	}
	return out
}

// prov is the provenance of a value node n builds (EVALUATION.md §13).
func (r *run) prov(n syntax.Node, kind value.ProvKind) *value.Prov {
	p := &value.Prov{Kind: kind, Span: r.span(n)}
	if f := r.fr; f.fn != "" || f.caller != nil {
		if !f.cached {
			all := r.frames()
			f.stack = all[:min(len(all), diag.MaxStackFrames)]
			f.more, f.cached = len(all)-len(f.stack), true
		}
		p.Stack, p.MoreFrames = f.stack, f.more
		if kind == value.ProvLiteral {
			p.Kind = value.ProvComputed
		}
	}
	return p
}
