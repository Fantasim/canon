package build

import (
	"context"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/wire"
)

// EditHost is a's host for an edit's values (edit.Env.Host; API.md V1, E6): a fresh evaluator
// of a's program whose loads and verifications report into throwaway bags, so the frozen
// analysis gains no finding; each call holds a's lock, which the evaluator's loader shares.
func EditHost(a *Analysis) wire.Host {
	a.mu.Lock()
	defer a.mu.Unlock()
	return editHost{a: a, h: a.r.throwawayHost()}
}

type editHost struct {
	a *Analysis
	h *evalHost
}

func (e editHost) Default(ctx context.Context, f *types.Field, in wire.Instance, via *value.Prov) (value.Value, bool) {
	e.a.mu.Lock()
	defer e.a.mu.Unlock()
	return e.h.ev.Default(ctx, f, in.Record, in.Params, via)
}

func (e editHost) Deref(ctx context.Context, r *value.Ref) (*value.Record, bool) {
	e.a.mu.Lock()
	defer e.a.mu.Unlock()
	return e.h.ev.Deref(ctx, r)
}

func (e editHost) Bind(rec *value.Record, params map[*types.Param]value.Value) {
	e.a.mu.Lock()
	defer e.a.mu.Unlock()
	e.h.ev.Bind(rec, params)
}

func (e editHost) Cycle(ctx context.Context, r *value.Ref) {
	e.a.mu.Lock()
	defer e.a.mu.Unlock()
	e.h.ev.Cycle(ctx, r)
}

func (e editHost) Reads(f *types.Field, fields []*types.Field) []int {
	e.a.mu.Lock()
	defer e.a.mu.Unlock()
	return e.h.ev.Reads(f, fields)
}

// Savepoint marks a decoding attempt; its end takes the lock again to undo or keep it.
func (e editHost) Savepoint() func(undo bool) {
	e.a.mu.Lock()
	end := e.h.ev.Savepoint()
	e.a.mu.Unlock()
	return func(undo bool) {
		e.a.mu.Lock()
		defer e.a.mu.Unlock()
		end(undo)
	}
}
