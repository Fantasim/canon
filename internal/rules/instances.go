package rules

import (
	"context"

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
	dt := r.declared[root]
	r.later(v, dt, root.Name)
	t := &traversal{Runner: r, ctx: ctx, bag: bag, root: root.Name, rootOf: root}
	t.alone = r.memo != nil && r.memo.alone(root)
	t.visit(v, dt)
	return nil
}

// traversal is one Instances run: where it stands, as the segments of the path of the value
// being visited, built only for a finding, and the memo's entry being recorded.
type traversal struct {
	*Runner
	ctx    context.Context
	bag    *diag.Bag
	root   string
	rootOf eval.Root
	alone  bool // no other top-level value traversed holds a part of this one (memo.go)
	segs   []seg
	rec    *entryRec
}

// visit walks v depth-first, pre-order: an instance's checks run before its parts are visited.
// It follows fields, elements, map keys then values, and entries, never refs.
func (t *traversal) visit(v value.Value, dt types.Type) {
	if rec, isRecord := v.(*value.Record); isRecord {
		if t.seen[rec] || t.ctx.Err() != nil {
			return
		}
		t.seen[rec] = true
		if t.rec != nil {
			t.rec.instance, t.rec.visited = t.rec.visited, t.rec.visited+1
		}
		t.runChecks(rec)
	}
	if tv, isTable := v.(*value.Table); isTable && t.alone && len(t.segs) == 0 {
		t.entries(tv)
		return
	}
	eachPart(v, dt, func(p part) bool {
		t.segs = append(t.segs, p.s)
		t.visit(p.v, p.t)
		t.segs = t.segs[:len(t.segs)-1]
		return true
	})
}

// here is the path of the value being visited.
func (t *traversal) here() *verify.Path {
	at := verify.Root(t.root)
	for _, s := range t.segs {
		at = s.on(at)
	}
	return at
}

// runChecks runs an instance's checks unless invalid below or broken, variant-level first (EVALUATION.md §7.3, §8.1).
func (t *traversal) runChecks(rec *value.Record) {
	if t.ask(rec) || t.brokenType(rec.T) {
		return
	}
	shared, own := t.checksOf(rec.T)
	for _, checks := range [...][]*syntax.CheckDecl{shared, own} {
		for _, c := range checks {
			if t.ctx.Err() != nil {
				return
			}
			run := t.run(c, rec)
			t.ran()
			t.report(c, run, rec, nil)
		}
	}
}

// run runs c on rec, traced for the memo while an entry is recorded.
func (t *traversal) run(c *syntax.CheckDecl, rec *value.Record) Run {
	if t.rec == nil {
		return t.ev.Run(t.ctx, c, rec)
	}
	x, trace := t.memo.ev.RunTraced(t.ctx, c, rec)
	run := runOf(x)
	t.memo.ev.Ran(c, rec, run)
	kept := &t.rec.kept
	switch {
	case trace == nil:
		t.rec.void = true
	case run.Failed:
		kept.failed = append(kept.failed, failedRun{c: c, run: len(kept.runs), instance: t.rec.instance, at: t.here()})
	}
	kept.runs = append(kept.runs, trace)
	return run
}

// checksOf is an instance's checks: its variant's variant-level ones, then its own (TYPES.md §12.1).
func (r *Runner) checksOf(t types.Type) (shared, own []*syntax.CheckDecl) {
	switch d := t.Base().(type) {
	case *types.RecordType:
		return nil, d.Checks
	case *types.CaseType:
		return r.shared[d.Variant], d.Checks
	case *types.AppliedRecord:
		return nil, d.Rec.Checks
	}
	return nil, nil
}
