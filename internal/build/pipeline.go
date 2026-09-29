package build

import (
	"context"
	"errors"
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/conform"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/i18n"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/load"
	"github.com/fantasim/canonlang/internal/lock"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/rules"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/verify"
	viewrules "github.com/fantasim/canonlang/internal/views/rules"
)

// run is one pass of phases 1 to 7 over a snapshot (EVALUATION.md §1).
type run struct {
	p         *Project
	s         *snapshot
	selected  []*project.Unit
	loaded    []*project.Unit  // the selected packages and their imports
	cps       []*check.Package // the selected packages, in package order
	bags      check.Bags
	fold      check.Folder
	prog      *check.Program
	opt       eval.Options // the evaluator's: the project budget, the active layers
	ev        *eval.Evaluator
	host      *evalHost
	assets    *assets
	vix       *verify.Index // built once, shared by every verifier of the run
	rix       *rules.Index  // idem, for the check runners
	order     []eval.Root   // the forced set, in order (EVALUATION.md §2.1)
	locks     []*lockState
	ir        []*ir.Package
	causes    bool                    // Analyze: the evaluator logs what poisons each root (Analysis.Cause)
	texts     map[string]*i18n.Result // phase 2's catalogues and translations, every package's
	failed    checkRuns               // stages C and D's failed named checks (VIEWMODEL.md J15)
	ownReads  *driverReads            // drivers read over the run's own program, once a model needs it
	wideReads *driverReads            // drivers read over the drivers-only program, idem
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
	loaded := withStudio(s.units, imported(s.units, selected), s.proj.Studio.Path)
	if err := p.checkLayers(s, loaded); err != nil {
		return nil, err
	}
	return &run{p: p, s: s, selected: selected, loaded: loaded, bags: s.bagsOf(loaded)}, nil
}

// analyze runs phases 2 to 7: check, then stages A to E (EVALUATION.md §1).
func (r *run) analyze(ctx context.Context) error {
	if err := r.check(ctx); err != nil {
		return err
	}
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
	r.host.loader.FinishDefines() // once every stage has forced its loads (WIRE.md §6.8)
	if err := ctx.Err(); err != nil {
		return err
	}
	return r.host.failure(r.s.set, r.prog)
}

// check is phase 2: resolve and type-check the loaded packages (EVALUATION.md §1).
func (r *run) check(ctx context.Context) error {
	r.opt = eval.Options{Budget: r.s.proj.Budget, Layers: r.p.opt.Layers}
	if err := r.phase2(ctx); err != nil {
		return err
	}
	r.dirConflicts()
	r.checkViews(ctx)
	return nil
}

// phase2 resolves and type-checks the loaded packages into r.bags, through the Checker seam if set.
func (r *run) phase2(ctx context.Context) error {
	r.fold = eval.NewFolder(r.bags, r.opt)
	files := filesOf(r.loaded)
	if r.p.opt.Checker != nil {
		r.prog = r.p.opt.Checker(ctx, r.s.proj, files, r.bags)
	} else {
		r.prog = check.Check(ctx, r.s.proj, files, r.bags, r.fold)
	}
	if r.prog == nil {
		return internal(errNoProgram)
	}
	return nil
}

// checkViews runs phase 2's static view and translation checks (EVALUATION.md §1 row 2, DECISIONS 221).
func (r *run) checkViews(ctx context.Context) {
	ev := r.emitsView()
	studio := r.s.proj.Studio.Path
	viewrules.Check(ctx, r.prog, r.bags, studio, ev)
	r.texts = i18n.Check(r.prog, r.s.proj, r.bags, ev)
}

// emitsView is the selected packages that declare `emit view` (I18N.md W1, VIEWMODEL.md N4).
func (r *run) emitsView() map[string]bool {
	out := map[string]bool{}
	for _, cp := range r.prog.Packages {
		if r.selects(cp.Path) && hasEmitView(cp.Files) {
			out[cp.Path] = true
		}
	}
	return out
}

// newHost is the run's host and evaluator, reporting into bags: stage A's, or canon test's.
func (r *run) newHost(bags check.Bags) {
	r.vix, r.rix = verify.NewIndex(r.prog), rules.NewIndex(r.prog)
	r.host = &evalHost{prog: r.prog, bags: bags, index: r.vix, fold: r.fold}
	r.host.loader = &load.Loader{FS: r.p.fs, Layout: r.s.layout, Set: r.s.set}
	r.ev = eval.New(r.prog, r.host, bags, r.opt)
	r.host.ev = r.ev
	r.assets = &assets{fs: r.p.fs, layout: r.s.layout, host: r.host, dirs: map[string]dirListing{}}
	r.host.assets = r.assets
	r.host.verifier = verify.NewShared(r.vix, r.ev, bags, r.assets)
}

// stageA forces every const and let of the selected packages in order (EVALUATION.md §2.1).
func (r *run) stageA(ctx context.Context) {
	r.newHost(r.bags)
	if r.causes {
		r.ev.LogCauses() // API.md R6
	}
	for _, cp := range r.prog.Packages {
		if !r.selects(cp.Path) {
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

// selectedNames are the selected packages' names, in name order.
func (r *run) selectedNames() []string {
	names := make([]string, len(r.selected))
	for i, u := range r.selected {
		names[i] = u.Name
	}
	return names
}

// selects reports a selected package.
func (r *run) selects(pkg string) bool {
	return slices.ContainsFunc(r.selected, func(u *project.Unit) bool { return u.Name == pkg })
}

// stageB verifies what stage A evaluated, then the codes, then compares the locks (LOCK.md §4.5).
func (r *run) stageB(ctx context.Context) error {
	r.ev.BeginVerification(ctx)
	if err := r.verifyCodes(); err != nil {
		return err
	}
	if err := r.compareLocks(ctx); err != nil {
		return err
	}
	r.checkUnits(ctx)
	return nil
}

// checkUnits reports an unknown unit name against the studio's evaluated `units` (EVALUATION.md §1 row 4, VIEWMODEL.md G16).
func (r *run) checkUnits(ctx context.Context) {
	studio := r.s.proj.Studio.Path
	if studio == "" {
		return
	}
	units, _ := r.ev.Force(ctx, eval.Root{Pkg: studio, Name: syntax.StudioUnits})
	viewrules.CheckUnits(ctx, r.prog, r.bags, studio, units)
}

// verifyCodes checks every @codes enum of every loaded package once (TYPES.md §8.1).
func (r *run) verifyCodes() error {
	for _, cp := range r.prog.Packages {
		for _, obj := range cp.Decls {
			if obj.Kind() != check.ObjTypeName || r.prog.Info.Broken[obj] {
				continue
			}
			if _, err := r.host.verifier.Codes(obj); err != nil {
				return internal(err)
			}
		}
	}
	return nil
}

// stagesCD runs the instance checks, then the package checks (EVALUATION.md §8).
func (r *run) stagesCD(ctx context.Context) error {
	r.failed.prog = r.prog
	runner := rules.NewShared(r.rix, checks{Evaluator: r.ev, failed: &r.failed}, r.bags)
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
	r.ir = ir.Build(ctx, ir.Input{
		Program: r.prog, Project: r.s.proj, Selected: r.selectedNames(), Bags: r.bags, Host: irHost{r.ev}, Fold: r.fold,
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
