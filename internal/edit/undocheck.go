package edit

import (
	"context"
	"slices"
)

// CheckUndo refuses an edit made with AllowErrors, once re-checked, whose Undo an operation would
// refuse (API.md E22; DECISIONS 273, 466): after is the re-check's analysis of plan's result, so
// it analyses nothing unless an inverse lands on a root after has no value of (E17a, NFR-01).
func CheckUndo(ctx context.Context, env Env, after *Snapshot, plan *Plan) error {
	if !slices.ContainsFunc(plan.Undo, func(op Operation) bool { return after.unvalued(op.Path) }) {
		return nil
	}
	b, err := newPlanner(ctx, env, after)
	if err != nil {
		return refusal(err)
	}
	b.replaying, b.allowErrors = true, true
	if err := b.replay(plan.Undo, after); err != nil {
		return plan.asked.undoRefusal(err)
	}
	return nil
}
