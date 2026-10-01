package edit

import (
	"context"

	"github.com/fantasim/canonlang/internal/value"
)

// DiffPaths are the paths where x and y differ as an Undo's verification compares them.
func DiffPaths(path string, x, y value.Value, ordered bool, placed func(string, *value.Record, *value.Record) bool) []string {
	d := &valueDiff{ordered: func(string) bool { return ordered }, placed: placed}
	d.value(path, x, y)
	return d.out
}

// LineText is a layer line's right-hand side as an Undo's verification compares it.
func LineText(text string) string { return flatText(text) }

// RootsCompared are the roots an Undo's verification compares, a Rename among ops or not.
func RootsCompared(base *Snapshot, ops []Operation, renamed bool, pkgs []string) []string {
	a := &applier{base: base, renamed: renamed}
	var out []string
	for _, p := range a.comparedRoots(ops, pkgs) {
		out = append(out, p.String())
	}
	return out
}

// PlanOverlay is Apply's plan and the overlay of the files it leaves: absolute name to bytes, nil absent.
func PlanOverlay(ctx context.Context, env Env, base *Snapshot, req Request) (*Plan, map[string][]byte, error) {
	p, err := Apply(ctx, env, base, req)
	if err != nil {
		return nil, nil, err
	}
	a := newApplier(ctx, env, base)
	a.multi = len(req.Ops) > 1
	for _, op := range req.Ops {
		if err := a.operation(op); err != nil {
			return nil, nil, err
		}
	}
	if err := a.cascade(); err != nil {
		return nil, nil, err
	}
	return p, a.overlay().files, nil
}
