package load

import (
	"context"

	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/wire"
)

// evalHost is wire.Host backed by the evaluator forcing the load (DECISIONS 173).
type evalHost struct {
	ev *eval.Evaluator
}

func (h evalHost) Default(ctx context.Context, f *types.Field, in wire.Instance, via *value.Prov) (value.Value, bool) {
	return h.ev.Default(ctx, f, in.Record, in.Params, via)
}

func (h evalHost) Deref(ctx context.Context, r *value.Ref) (*value.Record, bool) {
	return h.ev.Deref(ctx, r)
}

func (h evalHost) Bind(rec *value.Record, params map[*types.Param]value.Value) {
	h.ev.Bind(rec, params)
}

func (h evalHost) Cycle(ctx context.Context, r *value.Ref) {
	h.ev.Cycle(ctx, r)
}

func (h evalHost) Reads(f *types.Field, fields []*types.Field) []int {
	return h.ev.Reads(f, fields)
}

func (h evalHost) Savepoint() func(undo bool) {
	return h.ev.Savepoint()
}

// Through is req decoding through ev: its host, the let collection the load is the whole of, and
// what its type arguments name around it.
func (req Request) Through(ev *eval.Evaluator) Request {
	lc := ev.Loading()
	req.Host, req.Coll = evalHost{ev: ev}, lc.Coll
	req.Outer = wire.Outer{Record: lc.Record, Params: lc.Params, Binders: lc.Binders}
	return req
}
