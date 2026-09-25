package eval

import (
	"context"
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// loadSite is the load being decoded: the run forcing it, its expression, the let collection
// it is the whole value of (nil for none) and what its type arguments name.
type loadSite struct {
	r    *run
	at   syntax.Expr
	coll *types.Collection
	cx   *depCtx
	save *savepoint // the decoding attempt under way, nil for none
}

// savepoint is what a decoding attempt started from: the steps charged, the load's own, and
// the records it bound since.
type savepoint struct {
	outer        *savepoint
	steps, spent int64
	free         int64
	binds        []*value.Record
}

// LoadContext is what a decoder needs of the load being forced (DECISIONS 173).
type LoadContext struct {
	Coll    *types.Collection            // the let the load is the whole value of, nil for none
	Record  *value.Record                // the record whose field the load gives, nil for none
	Params  map[*types.Param]value.Value // that record's arguments
	Binders map[string]value.Value       // the dependent map binders around the load
}

// Loading is the context of the load being forced, zero outside one (TYPES.md §11.1).
func (e *Evaluator) Loading() LoadContext {
	if e.loading == nil {
		return LoadContext{}
	}
	lc := LoadContext{Coll: e.loading.coll}
	if cx := e.loading.cx; cx != nil {
		lc.Record, lc.Params, lc.Binders = cx.rec, cx.params, cx.binders
	}
	return lc
}

// Default is field f's default in self, a decoded record bound to params, run in the loading root (DECISIONS 173).
func (e *Evaluator) Default(ctx context.Context, f *types.Field, self *value.Record, params map[*types.Param]value.Value, via *value.Prov) (value.Value, bool) {
	e.bindParams(self, params)
	if f.Default == nil {
		return &value.None{T: f.Type, P: e.leftOut(f, self.T, via)}, true
	}
	r := e.decoding(ctx, e.declFile(self.T), "")
	v := r.defaultValue(self, f, nil, via)
	if v == nil || r.failed {
		return nil, false
	}
	return e.inField(v, self, f), true
}

// Deref is the entry ref names for a decode's branch, else E3501 or E3505 at the ref (DECISIONS 175, EVALUATION.md §3.2).
func (e *Evaluator) Deref(ctx context.Context, ref *value.Ref) (*value.Record, bool) {
	rt, ok := ref.T.Base().(*types.RefType)
	if !ok || rt.Target == nil {
		return nil, false
	}
	r := e.decoding(ctx, nil, rt.Target.Pkg)
	if e.loading != nil && rt.Target == e.loading.coll {
		r.refFail(diag.E3501.At(r.refSpan(ref, nil), ref, r.collName(rt.Target)), ref, nil)
		return nil, false
	}
	rec, ok := r.derefAt(ref, e.loadAt(), nil).(*value.Record)
	return rec, ok && !r.failed
}

// Savepoint marks a decoding attempt; end(true) takes back its steps and bindings (EVALUATION.md §12).
func (e *Evaluator) Savepoint() (end func(undo bool)) {
	ld := e.loading
	if ld == nil {
		return func(bool) {}
	}
	r := ld.r
	sp := &savepoint{outer: ld.save, steps: e.steps, spent: e.spent[r.charge], free: r.freeSteps}
	ld.save = sp
	return func(undo bool) {
		ld.save = sp.outer
		if !undo {
			if sp.outer != nil {
				sp.outer.binds = append(sp.outer.binds, sp.binds...)
			}
			return
		}
		for _, rec := range sp.binds {
			delete(e.bound, rec)
		}
		e.steps -= e.spent[r.charge] - sp.spent
		e.spent[r.charge], r.freeSteps = sp.spent, sp.free
		if sp.spent == 0 {
			e.order = slices.DeleteFunc(e.order, func(c charge) bool { return c == r.charge })
		}
	}
}

// Bind keeps the arguments a decoder bound to an applied record instance (TYPES.md §11.1).
func (e *Evaluator) Bind(rec *value.Record, params map[*types.Param]value.Value) {
	e.bindParams(rec, params)
}

// Cycle is E4301 at a loaded ref whose entry is needed to decode itself (EVALUATION.md §3.2).
func (e *Evaluator) Cycle(ctx context.Context, ref *value.Ref) {
	rt, ok := ref.T.Base().(*types.RefType)
	if !ok || rt.Target == nil {
		return
	}
	r := e.decoding(ctx, nil, rt.Target.Pkg)
	name := r.collName(rt.Target)
	r.refFail(diag.E4301.At(r.refSpan(ref, nil), []string{name, name}), ref, nil)
}

// Reads is the fields of fields, f's record, that f's default reads: bare names, `self.x`, and
// every one for a bare `self`.
func (e *Evaluator) Reads(f *types.Field, fields []*types.Field) []int {
	if out, ok := e.reads[f]; ok {
		return out
	}
	names, all := e.defaultNames(f.Default)
	var out []int
	for i, g := range fields {
		if all || slices.Contains(names, g.Name) {
			out = append(out, i)
		}
	}
	e.reads[f] = out
	return out
}

// defaultNames is the field names a default names: bare field names and `self.x`; all for a
// bare `self`.
func (e *Evaluator) defaultNames(d syntax.Expr) ([]string, bool) {
	var names []string
	all := false
	syntax.Inspect(d, func(n syntax.Node) bool {
		switch x := n.(type) {
		case *syntax.IdentExpr:
			if obj := e.info.Uses[x]; obj != nil && obj.Kind() == check.ObjField {
				names = append(names, x.Name)
			}
		case *syntax.SelectorExpr:
			if _, self := x.X.(*syntax.SelfExpr); self {
				names = append(names, x.Name.Name)
				return false
			}
		case *syntax.SelfExpr:
			all = true
		}
		return true
	})
	return names, all
}

// loadAt is the expression of the load being decoded, which a cycle it closes is reported at.
func (e *Evaluator) loadAt() syntax.Node {
	if e.loading == nil {
		return nil
	}
	return e.loading.at
}

// decoding is the run forcing the load being decoded; outside one, a run of its own in file,
// else in package pkg, whose bag takes its findings.
func (e *Evaluator) decoding(ctx context.Context, file *syntax.File, pkg string) *run {
	if e.loading != nil {
		return e.loading.r
	}
	r := e.newRun(ctx, charge{}, file)
	if file == nil {
		r.fr.pkg = pkg
	}
	r.charge.pkg = r.fr.pkg
	return r
}

// depAt is r's type-argument context with its dereferences reported at at.
func (r *run) depAt(at syntax.Node) *depCtx {
	cx := &depCtx{at: at}
	if r.dep != nil {
		cx.rec, cx.params, cx.binders, cx.field = r.dep.rec, r.dep.params, r.dep.binders, r.dep.field
	}
	return cx
}
