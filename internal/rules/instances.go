package rules

import (
	"context"
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/verify"
)

// Instances runs stage C over one top-level value, in forced-set order (EVALUATION.md §8.1).
func (r *Runner) Instances(ctx context.Context, root eval.Root, v value.Value) error {
	bag, err := r.bagOf(root.Pkg)
	if err != nil {
		return err
	}
	t := &traversal{Runner: r, ctx: ctx, bag: bag}
	at, dt := verify.Root(root.Name), r.declared[root]
	r.index(v, dt, at)
	t.visit(v, dt, at)
	return nil
}

// traversal is one Instances run.
type traversal struct {
	*Runner
	ctx context.Context
	bag *diag.Bag
}

// visit walks v depth-first, pre-order: an instance's checks run before its parts are visited.
// It follows fields, elements, map keys then values, and entries, never refs.
func (t *traversal) visit(v value.Value, dt types.Type, at *verify.Path) {
	if v == nil || t.ctx.Err() != nil {
		return
	}
	rec, isRecord := v.(*value.Record)
	if isRecord {
		if t.seen[rec] {
			return
		}
		t.seen[rec] = true
		t.runChecks(rec, at)
	}
	for _, p := range parts(v, dt, at) {
		t.visit(p.v, p.t, p.at)
	}
}

// index records where each value of the subtree is first reached, the path of its findings,
// as its top-level value is first traversed, before its checks can give a record an identity
// (verify's E5002_9 case).
func (r *Runner) index(v value.Value, dt types.Type, at *verify.Path) {
	if _, known := r.paths[v]; known || v == nil {
		return
	}
	r.paths[v] = at
	for _, p := range parts(v, dt, at) {
		r.index(p.v, p.t, p.at)
	}
}

// invalidBelow: v or a value under it, refs not followed, is invalid, asked at every instance and
// kept from the first asking, since a check run's soft finding can mark a value a later instance
// holds (verify's E5001_9 case, log-2026-09-29 M4 U13-r).
func (t *traversal) invalidBelow(v value.Value) bool {
	if v == nil {
		return false
	}
	if known, ok := t.below[v]; ok {
		return known
	}
	bad := t.ev.Invalid(v)
	for _, p := range parts(v, nil, nil) {
		if bad {
			break
		}
		bad = t.invalidBelow(p.v)
	}
	t.below[v] = bad
	return bad
}

// runChecks runs an instance's checks unless invalid below or broken, variant-level first (EVALUATION.md §7.3, §8.1).
func (t *traversal) runChecks(rec *value.Record, at *verify.Path) {
	if t.invalidBelow(rec) || t.brokenType(rec.T) {
		return
	}
	for _, c := range t.checksOf(rec.T) {
		if t.ctx.Err() != nil {
			return
		}
		t.report(c, t.ev.Run(t.ctx, c, rec), rec, at)
	}
}

func (r *Runner) checksOf(t types.Type) []*syntax.CheckDecl {
	switch d := t.Base().(type) {
	case *types.RecordType:
		return d.Checks
	case *types.CaseType:
		return append(slices.Clip(r.shared[d.Variant]), d.Checks...)
	case *types.AppliedRecord:
		return d.Rec.Checks
	}
	return nil
}
