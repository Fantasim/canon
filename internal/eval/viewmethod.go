package eval

import (
	"context"
	"maps"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// ViewMethod is View for the parameterless method a view's member item names, on self (VIEWMODEL.md §3.3, L21).
func (e *Evaluator) ViewMethod(ctx context.Context, name *syntax.Ident, self *value.Record) (value.Value, bool) {
	if e.prog == nil || e.info == nil || name == nil || self == nil || ctx.Err() != nil {
		return nil, false
	}
	obj := e.info.NameUses[name]
	if obj == nil || obj.Kind() != check.ObjMethod || e.broken(obj) { // EVALUATION.md §1; log-2026-09-29 M4 U9b-r
		return nil, false
	}
	if d, ok := obj.Decl().(*syntax.FnDecl); !ok || len(d.Params) != 0 {
		return nil, false
	}
	defer e.phase8(check.Bags{})()
	e.flush(context.WithoutCancel(ctx))
	r := e.newRun(ctx, charge{pkg: obj.Pkg()}, obj.File())
	r.free, r.fr.self = true, self
	v := r.invokeFn(fnCall{obj: obj, self: self, args: []value.Value{}})
	return v, v != nil && !r.failed
}

// BoundParams is a copy of the arguments bound to rec, nil for none (TYPES.md §11.1).
func (e *Evaluator) BoundParams(rec *value.Record) map[*types.Param]value.Value {
	return maps.Clone(e.boundParams(rec))
}
