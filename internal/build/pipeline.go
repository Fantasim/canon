package build

import (
	"context"
	"errors"
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/conform"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/load"
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
	opt      eval.Options // the evaluator's: the project budget, the active layers
	ev       *eval.Evaluator
	host     *evalHost
	assets   *assets
	vix      *verify.Index // built once, shared by every verifier of the run
	rix      *rules.Index  // idem, for the check runners
	order    []eval.Root   // the forced set, in order (EVALUATION.md §2.1)
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
		return internal(errNoProgram)
	}
	r.opt = opt
	r.stageA(ctx)
	if err := r.stageB(ctx); err != nil {
		return err
	}
	if err := r.stagesCD(ctx); err != nil {
		return err
	}
	if err := r.stageE(ctx); err != nil {
		return err
	}
	r.reportStableAmendments()
	if err := ctx.Err(); err != nil {
		return err
	}
	return r.host.failure(r.s.set, r.prog)
}

// stageA forces every const and let of the selected packages in order (EVALUATION.md §2.1).
func (r *run) stageA(ctx context.Context) {
	r.vix, r.rix = verify.NewIndex(r.prog), rules.NewIndex(r.prog)
	r.host = &evalHost{prog: r.prog, bags: r.bags, index: r.vix}
	r.host.loader = &load.Loader{FS: r.p.fs, Layout: r.s.layout, Set: r.s.set}
	r.ev = eval.New(r.prog, r.host, r.bags, r.opt)
	r.host.ev = r.ev
	r.assets = &assets{fs: r.p.fs, layout: r.s.layout, host: r.host, dirs: map[string][]string{}}
	r.host.assets = r.assets
	r.host.verifier = verify.NewShared(r.vix, r.ev, r.bags, r.assets)
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
	runner := rules.NewShared(r.rix, checks{r.ev}, r.bags)
	for _, root := range r.order {
		if v, ok := r.ev.Force(ctx, root); ok {
			if err := runner.Instances(ctx, root, v); err != nil {
				return internal(err)
			}
		}
	}
	for _, cp := range r.cps {
		if err := runner.Names(cp); err != nil {
			return internal(err)
		}
		if err := runner.Package(ctx, cp); err != nil {
			return internal(err)
		}
	}
	return nil
}

// stageE precomputes the export fns, validates the emits, then computes the vectors, in check and build alike (EVALUATION.md §1, §2.3, DECISIONS 37).
func (r *run) stageE(ctx context.Context) error {
	names := make([]string, len(r.selected))
	for i, u := range r.selected {
		names[i] = u.Name
	}
	r.ir = ir.Build(ctx, ir.Input{
		Program: r.prog, Project: r.s.proj, Selected: names, Bags: r.bags, Host: irHost{r.ev}, Fold: r.fold,
	})
	if err := untranslated(r.ir); err != nil {
		return err
	}
	err := conform.Fill(ctx, r.prog, r.ir, conformer{r}, r.bags)
	if err == nil {
		return nil
	}
	if cerr := ctx.Err(); cerr != nil {
		return cerr
	}
	if len(r.host.loads) > 0 { // an unsupported load explains an evaluation without an outcome (DECISIONS 196)
		return r.host.failure(r.s.set, r.prog)
	}
	return errors.Join(internal(err), r.host.failure(r.s.set, r.prog))
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
