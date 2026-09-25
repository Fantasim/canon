package build

import (
	"context"
	"errors"
	"fmt"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/conform"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/rules"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/verify"
)

// conformer is the evaluator as conform.Evaluator: tests on a fresh evaluator reporting into throwaway bags, vectors on stage A's (ADR-0003).
type conformer struct {
	r *run
}

// TestCalls runs pkg's tests as `canon test <pkg>` does, keeping fns' calls and, for the run, any compiler error (EVALUATION.md §1, CONFORMANCE.md §6.1).
func (c conformer) TestCalls(ctx context.Context, pkg string, fns []check.Object) []conform.Call {
	h := c.r.throwawayHost()
	calls, err := h.ev.TestCalls(ctx, pkg, fns, subjects{pkg: pkg, ev: h.ev, r: c.r, host: h})
	if err != nil {
		h.errs = append(h.errs, internal(err))
	}
	if err := h.ev.Err(); err != nil {
		h.errs = append(h.errs, internal(err))
	}
	c.r.host.absorb(h)
	out := make([]conform.Call, len(calls))
	for i, x := range calls {
		out[i] = conform.Call{Fn: x.Fn, Recv: x.Recv, Args: x.Args}
	}
	return out
}

// Evaluate runs one vector on its own cap, on a child of stage A's evaluator (CONFORMANCE.md §6.5).
func (c conformer) Evaluate(ctx context.Context, x conform.Call, m conform.Mode) conform.Outcome {
	o := c.r.ev.Vector(ctx, eval.Call{Fn: x.Fn, Recv: x.Recv, Args: x.Args}, eval.VectorMode{Steps: m.Steps, TS: m.TS})
	if int(o.Exceeded) >= len(limits) {
		c.r.host.errs = append(c.r.host.errs, internal(fmt.Errorf(fmtUnknownLimit, o.Exceeded)))
		return conform.Outcome{}
	}
	return conform.Outcome{Value: o.Value, Code: o.Code, Exceeded: limits[o.Exceeded]}
}

// throwawayHost is a fresh evaluator's host for test calls: its loads and verifications report
// into bags nothing reads, one per package of the program; its evaluator has the run's options.
func (r *run) throwawayHost() *evalHost {
	bags := r.throwawayBags()
	h := &evalHost{prog: r.prog, bags: bags, loader: r.host.loader, assets: r.assets, index: r.vix, scratch: true}
	h.ev = eval.New(r.prog, h, bags, r.opt)
	h.verifier = verify.NewShared(r.vix, h.ev, bags, r.assets)
	return h
}

// throwawayBags is one bag per package of the program, which nothing reads.
func (r *run) throwawayBags() check.Bags {
	bags := check.Bags{}
	for _, cp := range r.prog.Packages {
		bags[cp.Path] = diag.NewBag(r.s.set, cp.Path)
	}
	return bags
}

// absorb keeps what a throwaway host met that fails the run: its forced loads and its errors.
func (h *evalHost) absorb(o *evalHost) {
	if len(h.loads) == 0 {
		h.loadCause = o.loadCause
	}
	h.loads = append(h.loads, o.loads...)
	h.errs = append(h.errs, o.errs...)
}

// subjects builds an expect subject as `canon test` does, verified then instance-checked into its capture, its unbound refs' E3505 included (EVALUATION.md §10.2).
type subjects struct {
	pkg  string
	ev   *eval.Evaluator
	r    *run
	host *evalHost
}

func (s subjects) Build(ctx context.Context, v value.Value, capture *diag.Bag) {
	bags := check.Bags{}
	for _, cp := range s.r.prog.Packages {
		bags[cp.Path] = capture
	}
	root := eval.Root{Pkg: s.pkg}
	res, err := verify.NewShared(s.r.vix, s.ev, bags, s.r.assets).Check(ctx, root, v)
	if err != nil {
		s.host.errs = append(s.host.errs, internal(err))
		return
	}
	for _, u := range res.Unbound {
		s.ev.ReportUnboundInto(root, u.Ref, u.Path, capture)
	}
	if res.Poisoned { // a poisoned value is not instance-checked, as in stage C (EVALUATION.md §7.2)
		return
	}
	if err := rules.NewShared(s.r.rix, checks{s.ev}, bags).Instances(ctx, root, v); err != nil {
		s.host.errs = append(s.host.errs, internal(err))
	}
}

// untranslated is the internal error of every translated fn stage E left untranslated with no error reported (DECISIONS 196).
func untranslated(pkgs []*ir.Package) error {
	var errs []error
	for _, p := range pkgs {
		for _, fn := range exportFns(p) {
			if fn.Err != nil {
				errs = append(errs, internal(fn.Err))
			}
		}
	}
	return errors.Join(errs...)
}

// exportFns is every export fn of p: its types' methods in order, then its package fns.
func exportFns(p *ir.Package) []*ir.ExportFn {
	var out []*ir.ExportFn
	for _, t := range p.Types {
		switch t := t.(type) {
		case *ir.Record:
			out = append(out, t.Methods...)
		case *ir.Variant:
			for _, c := range t.Cases {
				out = append(out, c.Methods...)
			}
		}
	}
	return append(out, p.Fns...)
}
