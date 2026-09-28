package build

import (
	"context"
	"sync"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/value"
)

// Analysis is one run's checked program and evaluator, frozen once Analyze returns: no later call evaluates, spends budget or adds a finding.
type Analysis struct {
	mu  sync.Mutex
	r   *run
	res *Result
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
	return &Analysis{r: r, res: r.result()}, nil
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

// Force is root's value if stage A settled it, (nil, false) for any other; it never evaluates (EVALUATION.md §2.1).
func (a *Analysis) Force(root eval.Root) (value.Value, bool) {
	if !a.r.selects(root.Pkg) {
		return nil, false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.r.ev.Settled(root)
}

// Cause is the errors that poisoned root: its own, or the first poisoned value's it read, in any package (API.md R6, EVALUATION.md §7.2).
func (a *Analysis) Cause(root eval.Root) []diag.Finding {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.r.ev.PoisonCause(root).Findings
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
