package eval

import (
	"context"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
)

// VectorMode is how a conformance vector runs: its own step cap, TS integer checks (CONFORMANCE.md §4, §6.5).
type VectorMode struct {
	Steps int64 // the vector's own cap; 0: 1,000,000
	TS    bool
}

// Outcome is a vector's result, or the code of its first error, soft or hard, in the order
// reported; Exceeded is the limit that cut it short, if any.
type Outcome struct {
	Value    value.Value
	Code     diag.Code
	Exceeded Limit
}

// Limit is a limit that can cut a vector's evaluation short.
type Limit uint8

// vectorState is what a vector's evaluator keeps of its findings: the first error or limit.
type vectorState struct {
	first diag.Code
	limit Limit
	void  bool // a value it forced could not be verified: no outcome
	files diag.Files
}

// Vector evaluates c alone, on its own step cap and call depth, never the project budget; calls
// are sequential and e is not used meanwhile. It reads e's values, forces others afresh, charged
// to it and forgotten after (DECISIONS 204), reports nothing, and stops at its first error.
func (e *Evaluator) Vector(ctx context.Context, c Call, m VectorMode) Outcome {
	// CONFORMANCE.md §6.5
	if c.Fn == nil || e.broken(c.Fn) {
		return Outcome{}
	}
	d, ok := c.Fn.Decl().(*syntax.FnDecl)
	if !ok || c.Fn.File() == nil {
		return Outcome{}
	}
	ve := e.vectorEvaluator(m.Steps)
	r := ve.newRun(ctx, charge{pkg: c.Fn.Pkg(), name: c.Fn.Name()}, c.Fn.File())
	r.fr.ts = m.TS
	v := r.invokeFn(fnCall{obj: c.Fn, self: c.Recv, args: padded(c.Args, len(d.Params)), site: c.Fn.File().Span(d.Name)})
	e.bugs = append(e.bugs, ve.bugs...)
	return ve.vec.outcome(v, r.failed)
}

// vectorEvaluator is a child of e for one vector: its own budget, depth, forced values, invalid
// marks and index caches; e's program, host and interned caches shared.
func (e *Evaluator) vectorEvaluator(steps int64) *Evaluator {
	if steps <= 0 {
		steps = vectorCap
	}
	v := newEvaluator(check.Bags{}, Options{Budget: steps, Layers: e.opt.Layers})
	v.prog, v.info, v.index, v.pkgs = e.prog, e.info, e.index, e.pkgs
	if e.host != nil {
		v.host = &vectorHost{parent: e.host, ev: v}
	}
	v.regexps, v.frees, v.colls, v.fieldColls, v.pathFields = e.regexps, e.frees, e.interned(), e.fieldColls, e.pathFields
	v.ownedBy, v.sites, v.selfReads = e.ownedBy, e.sites, e.selfReads
	v.parent, v.vec, v.verifying = e, &vectorState{}, true
	v.whole = true // the vector's cap counts every package's steps (CONFORMANCE.md §6.5)
	return v
}

// cut stops the vector at limit l, the outcome unless an error came first.
func (e *Evaluator) cut(l Limit) {
	if s := e.vec; s.first == "" && s.limit == NoLimit {
		s.limit = l
	}
	e.exhausted = true
}

// note keeps the code of the vector's first error and stops the vector there.
func (s *vectorState) note(e *Evaluator, pkg string, b *diag.Builder) {
	if s.first != "" || s.limit != NoLimit {
		return
	}
	if s.files == nil {
		s.files = e.files()
	}
	one := diag.NewBag(s.files, pkg)
	b.Report(one)
	s.noteAll(e, one.Findings())
}

// noteAll keeps the code of the first error among fs, and stops the vector there.
func (s *vectorState) noteAll(e *Evaluator, fs []diag.Finding) {
	for _, f := range fs {
		if f.Severity == diag.Error && s.first == "" && s.limit == NoLimit {
			s.first, e.exhausted = f.Code, true
		}
	}
}

// outcome is the vector's outcome once its call returned v.
func (s *vectorState) outcome(v value.Value, failed bool) Outcome {
	switch {
	case s.void:
		return Outcome{}
	case s.limit != NoLimit:
		return Outcome{Exceeded: s.limit}
	case s.first != "":
		return Outcome{Code: s.first}
	case failed:
		return Outcome{}
	}
	return Outcome{Value: v}
}
