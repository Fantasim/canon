package build

import (
	"context"
	"fmt"
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/lock"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/rules"
	"github.com/fantasim/canonlang/internal/verify"
)

// run is one pass of phases 1 to 7 over a snapshot (EVALUATION.md §1).
type run struct {
	p        *Project
	s        *snapshot
	selected []*project.Unit
	loaded   []*project.Unit  // the selected packages and their imports
	cps      []*check.Package // the selected packages, in package order
	bags     check.Bags
	fold     check.Folder
	prog     *check.Program
	ev       *eval.Evaluator
	host     *evalHost
	order    []eval.Root // the forced set, in order (EVALUATION.md §2.1)
	locks    []*lockState
	ir       []*ir.Package
}

// Analyze runs phases 1 to 7 and writes nothing: what canon check reports (CLI.md §3.3).
func (p *Project) Analyze(ctx context.Context, selectors []string) (*Result, error) {
	r, err := p.prepare(ctx, selectors)
	if err != nil {
		return nil, err
	}
	if err := r.analyze(ctx); err != nil {
		return nil, err
	}
	return r.result(), nil
}

// prepare is phase 1: the snapshot parsed, the selection with its imports, the layers checked.
func (p *Project) prepare(ctx context.Context, selectors []string) (*run, error) {
	s, err := p.load(ctx)
	if err != nil {
		return nil, err
	}
	selected, err := project.Select(s.units, selectors)
	if err != nil {
		return nil, err
	}
	loaded := imported(s.units, selected)
	if err := p.checkLayers(loaded); err != nil {
		return nil, err
	}
	return &run{p: p, s: s, selected: selected, loaded: loaded, bags: s.bagsOf(loaded)}, nil
}

// analyze runs phases 2 to 7: check, then stages A to E (EVALUATION.md §1).
func (r *run) analyze(ctx context.Context) error {
	opt := eval.Options{Budget: r.s.proj.Budget, Layers: r.p.opt.Layers}
	r.fold = eval.NewFolder(r.bags, opt)
	if r.p.opt.Checker != nil {
		r.prog = r.p.opt.Checker(ctx, r.s.proj, filesOf(r.loaded), r.bags)
	} else {
		r.prog = check.Check(ctx, r.s.proj, filesOf(r.loaded), r.bags, r.fold)
	}
	if r.prog == nil {
		return fmt.Errorf(fmtNoProgram, ErrInternal)
	}
	r.stageA(ctx, opt)
	if err := r.stageB(ctx); err != nil {
		return err
	}
	if err := r.stagesCD(ctx); err != nil {
		return err
	}
	r.stageE(ctx)
	r.reportStableAmendments()
	if err := ctx.Err(); err != nil {
		return err
	}
	return r.host.failure(r.s.set, r.prog)
}

// stageA forces every const and let of the selected packages in order (EVALUATION.md §2.1).
func (r *run) stageA(ctx context.Context, opt eval.Options) {
	r.host = &evalHost{}
	r.ev = eval.New(r.prog, r.host, r.bags, opt)
	r.host.ev = r.ev
	a := &assets{fs: r.p.fs, layout: r.s.layout, host: r.host, dirs: map[string][]string{}}
	r.host.verifier = verify.New(r.ev, r.prog, r.bags, a)
	for _, cp := range r.prog.Packages {
		if !slices.ContainsFunc(r.selected, func(u *project.Unit) bool { return u.Name == cp.Path }) {
			continue
		}
		r.cps = append(r.cps, cp)
		for _, obj := range cp.Decls {
			if obj.Kind() == check.ObjConst || obj.Kind() == check.ObjLet {
				root := eval.Root{Pkg: cp.Path, Name: obj.Name()}
				r.order = append(r.order, root)
				r.ev.Force(ctx, root)
			}
		}
	}
}

// stageB verifies what stage A evaluated, then compares the locks (LOCK.md §4.5).
func (r *run) stageB(ctx context.Context) error {
	r.ev.BeginVerification(ctx)
	return r.compareLocks(ctx)
}

// stagesCD runs the instance checks, then the package checks (EVALUATION.md §8).
func (r *run) stagesCD(ctx context.Context) error {
	runner := rules.New(checks{r.ev}, r.prog, r.bags)
	for _, root := range r.order {
		if v, ok := r.ev.Force(ctx, root); ok {
			if err := runner.Instances(ctx, root, v); err != nil {
				return fmt.Errorf(fmtWrapInternal, ErrInternal, err)
			}
		}
	}
	for _, cp := range r.cps {
		if err := runner.Names(cp); err != nil {
			return fmt.Errorf(fmtWrapInternal, ErrInternal, err)
		}
		if err := runner.Package(ctx, cp); err != nil {
			return fmt.Errorf(fmtWrapInternal, ErrInternal, err)
		}
	}
	return nil
}

// stageE precomputes the export fns and validates the emits (EVALUATION.md §2.3).
func (r *run) stageE(ctx context.Context) {
	names := make([]string, len(r.selected))
	for i, u := range r.selected {
		names[i] = u.Name
	}
	r.ir = ir.Build(ctx, ir.Input{
		Program: r.prog, Project: r.s.proj, Selected: names, Bags: r.bags, Host: irHost{r.ev}, Fold: r.fold,
	})
}

// reportStableAmendments has lock report the amendments the evaluator refused (LOCK.md §6.1).
func (r *run) reportStableAmendments() {
	for _, a := range r.ev.StableAmendments() {
		lock.ReportLayer(lock.LayerAmendment{Layer: a.Layer, Table: a.Table, Field: a.Field, Span: a.Span}, r.s.bag(a.Pkg))
	}
}

// result is the findings of the selected packages and of the project itself, with the errors
// of the imported packages, which block the build (API.md R2, DECISIONS 196).
func (r *run) result() *Result {
	res := &Result{Revision: r.s.revision()}
	bags := make([]*diag.Bag, len(r.selected))
	for i, u := range r.selected {
		res.Packages = append(res.Packages, u.Name)
		bags[i] = r.s.bag(u.Name)
	}
	res.Findings = collect(r.s.set, r.s.own, bags...)
	for _, u := range r.loaded {
		if !slices.Contains(r.selected, u) {
			res.addErrors(r.s.bag(u.Name))
		}
	}
	return res
}
