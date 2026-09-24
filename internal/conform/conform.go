package conform

import (
	"context"
	"fmt"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/value"
)

// Evaluator is what conform asks of the one evaluator; build adapts eval to it (IMPLEMENTATION-PLAN §1).
type Evaluator interface {
	// TestCalls runs pkg's tests on a budget of their own, never the project's, discards their findings and returns their calls of fns made before any stop, in evaluation order (CONFORMANCE.md §6.1, DECISIONS 204).
	TestCalls(ctx context.Context, pkg string, fns []check.Object) []Call
	// Evaluate runs c alone on its own cap m.Steps, never the project budget, charging to it a value first forced inside c, its findings kept out of the bags (CONFORMANCE.md §4, §6.5, DECISIONS 204).
	Evaluate(ctx context.Context, c Call, m Mode) Outcome
}

// Call is one call of an export fn: its declaration, receiver (nil for a package fn) and arguments.
type Call struct {
	Fn   check.Object
	Recv value.Value
	Args []value.Value
}

// Mode is how a vector is evaluated: its step cap, and TypeScript's integer checks.
type Mode struct {
	Steps int64
	TS    bool
}

// Outcome is an evaluation's result, or the code of its first error, soft or hard, in the
// order reported; Exceeded is the limit that cut it short, if any.
type Outcome struct {
	Value    value.Value
	Code     diag.Code
	Exceeded Limit
}

// Limit is a limit that can cut an evaluation short.
type Limit uint8

// Fill fills the Vectors of each translated fn of pkgs with a code emit, reports E9008 and E9009
// to bags; build calls it in stage E, once the IR is built, before any generator.
func Fill(ctx context.Context, prog *check.Program, pkgs []*ir.Package, ev Evaluator, bags check.Bags) error {
	base := filler{ev: ev, errored: hasError(bags)}
	for _, p := range pkgs {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf(fmtCanceled, err)
		}
		if !hasTarget(p, isCode) { // CONFORMANCE.md §7.1: only a code emit translates (meta/decisions/log-2026-09-24.md)
			continue
		}
		base.bag = bags[p.Name]
		if err := fillPackage(ctx, prog, p, base); err != nil {
			return err
		}
	}
	return nil
}

// fillPackage fills the translated fns of one package, whose tests run once for all of them;
// base holds the evaluator, the package's bag and whether the bags held an error.
func fillPackage(ctx context.Context, prog *check.Program, p *ir.Package, base filler) error {
	sites, err := translated(prog, p)
	if err != nil || len(sites) == 0 {
		return err
	}
	if base.bag == nil {
		return fmt.Errorf(fmtNamed, ErrNoBag, p.Name)
	}
	// CONFORMANCE.md §4: only the TS conformance file reads TS expectations, so only a ts emit asks for them.
	f := &base
	f.ts, f.unknown = hasTarget(p, isTS), brokenTests(prog, p.Name)
	f.calls = f.ev.TestCalls(ctx, p.Name, objects(sites))
	for _, s := range sites {
		if err := f.fill(ctx, s); err != nil {
			return err
		}
	}
	return nil
}

// hasError reports an error finding in any bag: an evaluation may then meet a poisoned value.
func hasError(bags check.Bags) bool {
	for _, b := range bags { //canon:unordered a predicate over every bag
		if b != nil && b.Summary().Errors > 0 {
			return true
		}
	}
	return false
}

// isCode reports a target that translates export fns: go, cpp or ts.
func isCode(t ir.Target) bool {
	return t == ir.TargetGo || t == ir.TargetCpp || t == ir.TargetTS
}

// isTS reports the TypeScript target.
func isTS(t ir.Target) bool {
	return t == ir.TargetTS
}

// hasTarget reports an emit of p whose target keep accepts.
func hasTarget(p *ir.Package, keep func(ir.Target) bool) bool {
	for _, e := range p.Emits {
		if keep(e.Target) {
			return true
		}
	}
	return false
}
