package eval

import (
	"context"
	"fmt"
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
)

// Call is one call of a user fn: its declaration, receiver (nil for a package fn) and
// arguments, converted to the parameters' types.
type Call struct {
	Fn   check.Object
	Recv value.Value
	Args []value.Value
}

// recorder keeps the calls of some fns while TestCalls runs.
type recorder struct {
	fns   map[check.Object]bool
	calls []Call
}

// TestCalls runs pkg's tests as `canon test <pkg>` does, returning the calls of fns made before any
// stop, in order. e must be fresh, with project.budget and a host that verifies through e into
// throwaway bags, never the project's; e drops its own findings, b captures the subjects'.
func (e *Evaluator) TestCalls(ctx context.Context, pkg string, fns []check.Object, b Builder) ([]Call, error) {
	// CONFORMANCE.md §6.1, EVALUATION.md §1, §10.1
	if !e.fresh() {
		return nil, ErrNotFresh
	}
	cp := e.pkgs[pkg]
	if cp == nil {
		return nil, fmt.Errorf(fmtWrap, ErrNoPackage, pkg)
	}
	rec := &recorder{fns: map[check.Object]bool{}}
	for _, fn := range fns {
		rec.fns[fn] = true
	}
	saved := e.bags
	e.bags, e.recording = check.Bags{}, rec
	defer func() { e.bags, e.recording = saved, nil }()
	e.BeginVerification(ctx)
	for _, obj := range cp.Decls {
		t, ok := obj.Decl().(*syntax.TestDecl)
		if !ok || obj.Kind() != check.ObjTest {
			continue
		}
		if e.exhausted || ctx.Err() != nil {
			break
		}
		e.Test(ctx, t, b)
	}
	return rec.calls, nil
}

// fresh reports an evaluator that has evaluated nothing yet.
func (e *Evaluator) fresh() bool {
	if e.parent != nil || e.steps != 0 || e.exhausted || e.verifying || len(e.stack) != 0 {
		return false
	}
	for _, st := range e.states { //canon:unordered a predicate over every state
		if st.status != idle {
			return false
		}
	}
	return true
}

// record keeps a call of a recorded fn once its arguments are converted.
func (e *Evaluator) record(c fnCall) {
	rec := e.recording
	if rec == nil || !rec.fns[c.obj] || slices.Contains(c.args, nil) {
		return
	}
	rec.calls = append(rec.calls, Call{Fn: c.obj, Recv: c.self, Args: slices.Clone(c.args)})
}
