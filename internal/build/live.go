package build

import (
	"context"

	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/views/render"
)

// ViewEvaluator is the analysis's evaluator for Evaluate's templates, one at a time: its loads and
// verifications report into throwaway bags, so the frozen analysis gains no finding and a
// failure is only false (API.md V11, V13).
func (a *Analysis) ViewEvaluator() render.Evaluator {
	return liveEval{a}
}

// ViewErr is the internal error or unsupported load ViewEvaluator's evaluations met, nil when
// none did: a failure of the compiler, which Evaluate reports as such (API.md X2).
func (a *Analysis) ViewErr() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.viewErr
}

type liveEval struct {
	a *Analysis
}

func (l liveEval) Eval(ctx context.Context, e syntax.Expr, self value.Value, m render.Magic) (value.Value, bool) {
	l.a.mu.Lock()
	defer l.a.mu.Unlock()
	r := l.a.r
	restore := r.hostAside(r.throwawayBags())
	v, ok := viewEval{r.ev}.Eval(ctx, e, self, m)
	if err := restore(); err != nil { // the run's failure so far: kept, never swallowed
		l.a.viewErr = err
		return nil, false
	}
	return v, ok
}
