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
	ev         *Evaluator
	ctx        context.Context
	charge     charge
	fr         *frame
	failed     bool
	tainted    bool
	starved    bool      // aborted by a spent budget, its package's or that of a value it read
	free       bool      // stage B's where re-runs cost nothing (DECISIONS 148)
	sink       *diag.Bag // captures the findings of an expect subject
	site       source.Span
	chainNone  bool
	at         *vpath
	cur        *vpath // the value path of the literal being built, for a hard error's Path (API.md F1)
	instPath   string // a check run's instance path, for a hard error's Path (API.md F1)
	coll       *collHint
	reports    *[]CheckReport
	poisonAt   string
	poisonRoot Root        // the value poisonAt names
	poisonSpan source.Span // where the run read it
	root       *rootState  // the top-level value this run evaluates, if it is one
	cmp        *comparison // the comparison of the expect being evaluated, if any
	h          *stdHost
	test       *testState
	freeSteps  int64
	mv         *moves           // what the amendments of this root copied, until settled (settle.go)
	dep        *depCtx          // what a type argument names in the value being built (params.go)
	emitted    *[]*diag.Builder // a stage-B run's findings, kept for replay (stageb.go)
	magic      *Magic           // a view run's magic names (viewexpr.go)
	outer      *diag.Frame      // a precomputation's frame its findings carry (EVALUATION.md §2.3)
	notesCuts  bool             // a precomputation: its cut stacks keep their outermost frame (outermost.go)
	peek       syntax.Expr      // the receiver being read as a peek, until that read (owned.go)
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
	named  *frame // the innermost frame at or above this one that has a fn
	top    *frame // the outermost frame at or above this one that has a fn
	count  int    // the frames with a fn at or above this one
	ret    value.Value
	stack  []diag.Frame
	more   int
	cached bool
	decl   bool                         // an implicit frame: a field default or a where run (DECISIONS 210)
	ts     bool                         // a translated fn's frame in a TS-mode vector (CONFORMANCE.md §4)
	reads  map[syntax.Expr]selfRead     // its paths of self, read on entry in TS mode
	owns   map[check.Object]value.Value // the vars' collections no one else holds (owned.go)
}

// under links f below caller, counting its user frames; it returns f.
func (f *frame) under(caller *frame) *frame {
	f.caller = caller
	if caller != nil {
		f.named, f.count, f.top = caller.named, caller.count, caller.top
	}
	if f.fn != "" {
		f.named, f.count = f, f.count+1
		if f.top == nil {
			f.top = f
		}
	}
	return f
}

// above is the next frame with a fn above a named frame f.
func (f *frame) above() *frame {
	if f.caller == nil {
		return nil
	}
	return f.caller.named
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
		return r.spendFree(n)
	case e.spentOut(r.charge.pkg): // that package's evaluation stopped at its E4401
		r.failed, r.starved = true, true
		return false
	case n == 0:
		return true
	}
	before := e.steps
	if e.pay(r.charge, int64(n)) {
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
// cancellation, and past a count the size of the budget it aborts that run alone, poisoned,
// without E4401 (DECISIONS 195).
func (r *run) spendFree(n int) bool {
	before := r.freeSteps
	r.freeSteps += int64(n)
	if r.freeSteps >= r.ev.budget {
		r.failed = true
		return false
	}
	if before/cancelEvery != r.freeSteps/cancelEvery && r.ctx.Err() != nil {
		r.ev.exhausted, r.failed = true, true
		return false
	}
	return true
}

// remaining is the steps the run may spend before its package's budget, or a free run's cap, runs out.
func (r *run) remaining() int {
	if r.free {
		return int(max(r.ev.budget-r.freeSteps, 0))
	}
	e := r.ev
	return int(max(e.budget-e.perOf(r.charge.pkg), 0))
}

// budgetOut is E4401, among the findings of the run's package, whose evaluation alone stops (EVALUATION.md §12.2).
func (r *run) budgetOut(at source.Span) {
	e, pkg := r.ev, r.charge.pkg
	r.failed, r.starved = true, true
	if e.vec != nil {
		e.out[e.key(pkg)] = nil
		e.cut(StepLimit)
		return
	}
	heavy := e.heaviest(pkg)
	b := r.withStack(diag.E4401.At(at, e.budget, qualify(pkg, heavy.declPkg(), heavy.name), e.spentOn(heavy))).Path(r.findingPath())
	e.out[pkg] = b
	e.noteOut(r, b)
	e.reported = append(e.reported, pkg)
	e.report(pkg, b)
}

// fail reports a hard error with the root's call stack and aborts the root.
func (r *run) fail(b *diag.Builder) {
	r.abort(r.withStack(b))
}

// abort reports a hard error and aborts the root; a tainted root aborts silently (§7.3).
func (r *run) abort(b *diag.Builder) {
	r.abortAt(b, r.findingPath())
}

// abortAt is abort with the finding about the value at path (API.md F1).
func (r *run) abortAt(b *diag.Builder, path string) {
	if r.failed {
		return
	}
	r.failed = true
	r.noteAbort()
	if !r.tainted || r.sink != nil || r.ev.vec != nil {
		b.Path(path)
		r.noteStop(b)
		r.emit(b)
	}
}

// findingPath is the path of the value a hard error of this run is about: the innermost literal
// being built, else the instance a check runs on, else the top-level value the run evaluates,
// "" for a run of none (API.md F1).
func (r *run) findingPath() string {
	switch {
	case r.cur != nil:
		return r.cur.String()
	case r.instPath != "":
		return r.instPath
	}
	return r.wholePath()
}

// wholePath is the path of the whole value the run evaluates: the instance a check runs on, else
// the top-level value, "" for a run of none. A finding about the call chain, not a part, takes it.
func (r *run) wholePath() string {
	switch {
	case r.instPath != "":
		return r.instPath
	case r.root != nil:
		return rootPath(r.root.obj.Name()).String()
	}
	return ""
}

// stop aborts the root without a finding: a poisoned read (EVALUATION.md §7.2).
func (r *run) stop() {
	r.failed = true
}

// emit reports a finding into the capture of an expect, else its package's bag (a fold's
// one bag while folding).
func (r *run) emit(b *diag.Builder) {
	if r.emitted != nil {
		*r.emitted = append(*r.emitted, b)
	}
	if r.sink != nil {
		b.Report(r.sink)
		return
	}
	r.noteFinding(b)
	r.ev.report(r.fr.pkg, b)
}

// soft reports a soft finding about v and marks it invalid (EVALUATION.md §4.3, §7.1).
func (r *run) soft(b *diag.Builder, v value.Value, at *vpath) {
	var stack []diag.Frame
	more := 0
	if p := origin(v.Prov()); p != nil {
		b.Pointer(p.Pointer).Layer(p.Layer)
		stack, more = p.Stack, p.MoreFrames
	}
	stack, more = Outermost(stack, more, r.outer, r.ev.Hides(stack, r.outer))
	r.emit(b.Stack(stack).MoreFrames(more).Path(at.String()))
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
	if v != nil && r.ev.Invalid(v) {
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
	return qualify(r.fr.pkg, pkg, name)
}

// frames is the call stack, innermost first, cut to diag.MaxStackFrames, and the count cut (EVALUATION.md §13).
func (r *run) frames() ([]diag.Frame, int) {
	out := make([]diag.Frame, 0, min(r.fr.count, diag.MaxStackFrames))
	for f := r.fr.named; f != nil && len(out) < diag.MaxStackFrames; f = f.above() {
		out = append(out, diag.Frame{Fn: f.fn, Span: f.call})
	}
	return out, r.fr.count - len(out)
}

// withStack is b with the current call stack.
func (r *run) withStack(b *diag.Builder) *diag.Builder {
	stack, more := r.frames()
	stack, more = Outermost(stack, more, r.outer, false) // a run under outer never holds its frame
	return b.Stack(stack).MoreFrames(more)
}

// prov is the provenance of a value node n builds; a literal in a default stays one (EVALUATION.md §13).
func (r *run) prov(n syntax.Node, kind value.ProvKind) *value.Prov {
	p := &value.Prov{Kind: kind, Span: r.span(n)}
	if f := r.fr; (f.fn != "" || f.caller != nil) && (!f.decl || kind != value.ProvLiteral) {
		if !f.cached {
			f.stack, f.more = r.frames()
			f.cached = true
			r.noteCut(f)
		}
		p.Stack, p.MoreFrames = f.stack, f.more
		if kind == value.ProvLiteral {
			p.Kind = value.ProvComputed
		}
	}
	return p
}
