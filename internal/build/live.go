package build

import (
	"context"
	"maps"
	"slices"

	"github.com/fantasim/canonlang/internal/i18n"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/views/render"
)

// ViewEvaluator is the analysis's evaluator for Evaluate's templates, one at a time: its loads and
// verifications report into throwaway bags, so the frozen analysis gains no finding and a
// failure is only false (API.md V11, V13).
func (a *Analysis) ViewEvaluator() render.Evaluator {
	return liveEval{a}
}

// ViewMethods is ViewEvaluator for the methods views name (API.md V10, VIEWMODEL.md L21).
func (a *Analysis) ViewMethods() render.MethodEvaluator {
	return liveEval{a}
}

// ViewBound is the arguments each applied record keeps, by parameter (TYPES.md 11.1).
func (a *Analysis) ViewBound() func(*value.Record) map[*types.Param]value.Value {
	return func(rec *value.Record) map[*types.Param]value.Value {
		a.mu.Lock()
		defer a.mu.Unlock()
		return a.r.ev.BoundParams(rec)
	}
}

// LiveInputs are what Evaluate reads besides the program and the settled values (API.md 11).
type LiveInputs struct {
	Studio    string
	Languages []string                // the source first
	I18N      map[string]*i18n.Result // phase 2's, by package, read-only
}

// LiveInputs are the ones Analyze computed, copied.
func (a *Analysis) LiveInputs() LiveInputs {
	proj := a.r.s.proj
	return LiveInputs{Studio: proj.Studio.Path, Languages: slices.Clone(proj.Languages), I18N: maps.Clone(a.r.texts)}
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
	return l.aside(func() (value.Value, bool) { return viewEval{l.a.r.ev}.Eval(ctx, e, self, m) })
}

func (l liveEval) Method(ctx context.Context, name *syntax.Ident, self *value.Record) (value.Value, bool) {
	return l.aside(func() (value.Value, bool) { return l.a.r.ev.ViewMethod(ctx, name, self) })
}

// aside runs one evaluation of the frozen analysis with its host's findings set aside.
func (l liveEval) aside(run func() (value.Value, bool)) (value.Value, bool) {
	l.a.mu.Lock()
	defer l.a.mu.Unlock()
	r := l.a.r
	restore := r.hostAside(r.throwawayBags())
	v, ok := run()
	if err := restore(); err != nil { // the run's failure so far: kept, never swallowed
		l.a.viewErr = err
		return nil, false
	}
	return v, ok
}
