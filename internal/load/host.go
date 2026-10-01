package load

import (
	"context"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/wire"
)

// Request is a load forced from one file: package, directory, site, bag, decoding host (DECISIONS 173).
type Request struct {
	Pkg     string
	From    string
	Span    source.Span
	Bag     *diag.Bag
	Host    wire.Host
	Coll    *types.Collection // the let collection the load is the whole value of (wire.Decoder.Coll)
	Outer   wire.Outer        // what the loaded type's arguments name around the load
	Field   *types.Field      // the field scope the load decodes in, nil for none (wire.Decoder.Field, DECISIONS 268)
	Scratch bool              // Bag is thrown away (a test call's, a vector's): the Loader caches nothing for it
	found   *[]*diag.Builder  // where a recorded load keeps its own findings (Recorded)
}

// decoder is the wire decoder of req's load.
func (req Request) decoder(partial bool) *wire.Decoder {
	return &wire.Decoder{Bag: req.Bag, Pkg: req.Pkg, Host: req.Host, Partial: partial, Coll: req.Coll, Outer: req.Outer, Field: req.Field}
}

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

// Through is req decoding through ev: its host, the let collection the load is the whole of, what
// its type arguments name around it, and the field scope it decodes in.
func (req Request) Through(ev *eval.Evaluator) Request {
	lc := ev.Loading()
	req.Host, req.Coll, req.Field = evalHost{ev: ev}, lc.Coll, lc.Field
	req.Outer = wire.Outer{Record: lc.Record, Params: lc.Params, Binders: lc.Binders}
	return req
}
