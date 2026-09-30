package build

import (
	"context"
	"fmt"
	"slices"
	"sync"

	"github.com/fantasim/canonlang/api/vm"
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/value"
)

// Analysis is one run's checked program and evaluator, frozen once Analyze returns: no later
// call spends budget or adds a finding; ViewModel alone evaluates, as phase 8 does.
type Analysis struct {
	mu      sync.Mutex
	r       *run
	res     *Result
	settled map[eval.Root]value.Value // the selected packages' values settled when Analyze returned
	viewErr error                     // what ViewEvaluator's evaluations failed with, but a template's own failure
	causes  *eval.Evaluator           // a memoized run's causes, logged on first use (cause.go)
	inputs  inputs                    // what Analyze's run read, which Manifest lists
	listed  []byte                    // the manifest, made on first use
}

// Analyze runs phases 1 to 7, writes nothing, and freezes the result (CLI.md §3.3).
func (p *Project) Analyze(ctx context.Context, selectors []string) (*Analysis, error) {
	r, err := p.prepare(ctx, selectors)
	if err != nil {
		return nil, err
	}
	r.causes = true
	if err := r.analyze(ctx); err != nil {
		return nil, err
	}
	return r.analysis(), nil
}

// analysis is the analyzed run frozen: its findings, settled values and manifest.
func (r *run) analysis() *Analysis {
	return &Analysis{r: r, res: r.result(), settled: r.settledRoots(), inputs: r.inputs()}
}

// settledRoots is every const and let of the selected packages that phases 3-7 settled.
func (r *run) settledRoots() map[eval.Root]value.Value {
	out := map[eval.Root]value.Value{}
	for _, cp := range r.prog.Packages {
		if !r.selects(cp.Path) {
			continue
		}
		for _, obj := range cp.Decls {
			root := eval.Root{Pkg: cp.Path, Name: obj.Name()}
			if v, ok := r.ev.Settled(root); ok && (obj.Kind() == check.ObjConst || obj.Kind() == check.ObjLet) {
				out[root] = v
			}
		}
	}
	return out
}

// Manifest is the build manifest of what Analyze read, made on first call, its globs matched then (WIRE.md §10).
func (a *Analysis) Manifest() []byte {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.listed == nil {
		a.listed = a.r.manifest(a.inputs, commandCheck, nil)
	}
	return slices.Clone(a.listed)
}

// Result is the findings this Analyze reported (API.md R2).
func (a *Analysis) Result() *Result { return a.res }

// Program is the checked program of the loaded packages (§4.7).
func (a *Analysis) Program() *check.Program { return a.r.prog }

// Bag is pkg's own phases-1-7 findings, nil unless Analyze selected pkg (VIEWMODEL.md J15).
func (a *Analysis) Bag(pkg string) *diag.Bag {
	if !a.r.selects(pkg) {
		return nil
	}
	return a.r.s.bags[pkg]
}

// Files locates every span Bag or Result reports.
func (a *Analysis) Files() diag.Files { return a.r.s.set }

// Layout is the project's roots as this analysis resolved its paths, read-only (VIEWMODEL.md 12.3 `asset`).
func (a *Analysis) Layout() *project.Layout { return a.r.s.layout }

// Force is root's value if Analyze settled it, (nil, false) for any other, whatever ViewModel evaluated since (EVALUATION.md §2.1).
func (a *Analysis) Force(root eval.Root) (value.Value, bool) {
	v, ok := a.settled[root]
	return v, ok
}

// ViewModel is selected pkg's view model as `emit view` writes it, what it reads evaluated aside (API.md R9, VIEWMODEL.md §12).
func (a *Analysis) ViewModel(ctx context.Context, pkg string) (*vm.ViewModel, error) {
	if !a.r.selects(pkg) {
		return nil, fmt.Errorf(fmtPackage, pkg, ErrNotSelected)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.r.viewModel(ctx, pkg)
}

// Produced is v as its origin produced it, before later amendments of its descendants copied it (CLI.md §3.7).
func (a *Analysis) Produced(v value.Value) value.Value {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.r.ev.Produced(v)
}

// History is path's last value, then each value amendments replaced, newest first (CLI.md §3.7).
func (a *Analysis) History(path ...value.Value) []value.Value {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.r.ev.History(path...)
}
