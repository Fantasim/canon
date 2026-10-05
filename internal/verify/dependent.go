package verify

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// depSite is the dependent value being judged.
type depSite struct {
	app      *types.TypeAppType // the application written for it
	field    string             // the field it is given to directly, "" for none
	optional bool               // that field is optional
	whole    types.Type         // the type the application computes, nested applications computed
}

// branchKey is one application evaluated in one env: once per value (TYPES.md §11.6).
type branchKey struct {
	app *types.TypeAppType
	e   *env
}

// branchOut is what an application computed: its type and the env binding its parameters.
type branchOut struct {
	t     types.Type
	inner *env
}

// dependent judges v against the type app computes in sc's env (TYPES.md §11.6).
func (w *walker) dependent(v value.Value, app *types.TypeAppType, optional bool, at *Path, sc scope) value.Value {
	w.voidRec()
	t, inner, ok := w.branch(app, sc.env)
	if !ok {
		return v
	}
	switch {
	case sc.dep == nil:
		sc.dep, sc.direct = &depSite{app: app, field: sc.field, optional: optional, whole: w.substitute(t, inner)}, true
	case optional && !sc.dep.optional:
		d := *sc.dep
		d.optional = true
		sc.dep = &d
	}
	sc.env = inner
	return w.judge(v, t, at, sc)
}

// branch is the type app computes in e, charged once per env (EVALUATION.md §12.1).
func (w *walker) branch(app *types.TypeAppType, e *env) (types.Type, *env, bool) {
	key := branchKey{app: app, e: e}
	if out, ok := w.branches[key]; ok {
		return out.t, out.inner, true
	}
	if w.cachedOnly {
		return nil, nil, false
	}
	fn := app.Fn
	switch {
	case len(fn.Params) != len(app.Args):
		return w.unjudged(reasonArity, app)
	case w.stage == nil:
		return w.unjudged(reasonNoStage, app)
	case !w.charge(app):
		return nil, nil, false
	}
	inner, none, ok := w.args(app, e)
	if !ok {
		return nil, nil, false
	}
	t := types.NeverType // an argument that reads none computes Never, its body unevaluated (TYPES.md §11.6)
	if !none {
		if t, ok = w.selected(fn, inner); !ok {
			return nil, nil, false
		}
	}
	w.branches[key] = branchOut{t: t, inner: inner}
	return t, inner, true
}

// args binds app's arguments read in e, and reports one that reads none (TYPES.md §11.1).
func (w *walker) args(app *types.TypeAppType, e *env) (inner *env, none, ok bool) {
	fn := app.Fn
	inner = &env{up: e, params: make(map[*types.Param]value.Value, len(fn.Params))}
	for i, p := range fn.Params {
		v, r := w.arg(app.Args[i], e)
		if !w.settled(r, app) {
			return nil, false, false
		}
		inner.params[p] = v
		none = none || isNone(v)
	}
	return inner, none, true
}

// selected is the arm fn's scrutinee selects in inner, or its body.
func (w *walker) selected(fn *types.TypeFunc, inner *env) (types.Type, bool) {
	s := fn.Scrutinee
	if s == nil {
		return fn.Body, fn.Body != nil
	}
	v, r := w.follow(inner.params[s.Param], s.Path)
	if !w.settled(r, nil) {
		return nil, false
	}
	i, ok := value.ArmIndex(v)
	if !ok || fn.Arm(i) == nil {
		_, _, ok = w.unjudged(reasonArm, nil)
		return nil, ok
	}
	return fn.Arm(i).Result, true
}

// settled reports a read that gave a value; a failed dereference aborts the root (EVALUATION.md §5).
func (w *walker) settled(r read, app *types.TypeAppType) bool {
	switch r {
	case aborted:
		w.res.Poisoned, w.res.Valid = true, false
	case missing:
		w.unjudged(reasonMissing, app)
	case found:
	}
	return r == found
}

// unjudged stops verification with ErrUnjudged: a dependent value is never handed on unconverted.
func (w *walker) unjudged(reason string, app *types.TypeAppType) (types.Type, *env, bool) {
	name := ""
	if app != nil {
		name = app.String()
	}
	w.err, w.res.Valid = fmt.Errorf(fmtUnjudged, ErrUnjudged, reason, name), false
	return nil, nil, false
}

// charge spends a step for app and one per node of its argument and scrutinee paths (EVALUATION.md §12.1).
func (w *walker) charge(app *types.TypeAppType) bool {
	n := 1
	for _, a := range app.Args {
		n += len(a.Path)
		if a.Source != types.ArgField {
			n++ // the parameter or binder a path starts from; a field's path holds the field
		}
	}
	if s := app.Fn.Scrutinee; s != nil {
		n += 1 + len(s.Path)
	}
	w.charged++
	if w.stage.Charge(n, w.src.types[app].typ) {
		return true
	}
	w.halted, w.res.Valid = true, false
	return false
}

// judge converts v to the computed type t, then verifies it as any value of t (TYPES.md §11.6).
func (w *walker) judge(v value.Value, t types.Type, at *Path, sc scope) value.Value {
	base, optional, ok := layers(v, t, nil)
	switch {
	case !ok || base == nil:
		return v
	case base.Kind() == types.TypeApp:
		return w.dependent(v, base.(*types.TypeAppType), optional, at, sc)
	case isNone(v):
		w.none(v, base, optional, at, sc)
		return v
	case base.Kind() == types.Never:
		w.never(v, optional, at, sc)
		return v
	}
	nv, fit := eval.ToBranch(v, base, w.stage.Written(v))
	switch fit {
	case eval.NoFit:
		w.mismatch(v, at, sc.dep)
		return v
	case eval.BareCase:
		if nv = w.bareCase(v, nv.(*value.Record), at, sc.dep); nv == nil {
			return v
		}
	case eval.Fits:
	}
	if ref, ok := nv.(*value.Ref); ok && nv != v && ref.T.Base().(*types.RefType).Target.Kind == types.CollField {
		ref.Owner = sc.env.owner(ref.T.Base().(*types.RefType).Target) // its root's instance (EVALUATION.md §3.4)
	}
	return w.stored(v, nv, t, at, sc)
}

// stored is nv, v converted, stored at t as any storage point stores it, then verified; a
// container is retyped to t, its parts judged one by one, then retyped to t as they computed it.
func (w *walker) stored(v, nv value.Value, t types.Type, at *Path, sc scope) value.Value {
	if eval.IsContainer(nv) {
		nv = w.retyped(nv, t)
		n := len(w.stage.Emitted())
		ok := w.stage.Refine(nv, t, w.src.types[sc.dep.app].decl, at.String())
		if w.keepSince(n, at.String()); !ok {
			w.res.Poisoned, w.res.Valid = true, false
			return v
		}
		if base, _, ok := layers(nv, t, nil); ok {
			return w.retyped(w.dispatch(nv, base, at, sc.part()), w.computed(t, sc.env))
		}
		return nv
	}
	n := len(w.stage.Emitted())
	s, ok := w.stage.Store(nv, t, w.src.types[sc.dep.app].decl, at.String())
	w.keepSince(n, at.String())
	if !ok {
		w.res.Poisoned, w.res.Valid = true, false
		return v
	}
	if s != v {
		w.moved(v, s)
	}
	nsc := sc.part()
	nsc.dep, nsc.past = nil, sc.past // a past application's selected branch is past (TYPES.md §8.4)
	return w.walk(s, t, at, nsc)
}

// none judges none: fine for an optional field or type, E3801 or E3802 otherwise (TYPES.md §11.6).
func (w *walker) none(v value.Value, base types.Type, optional bool, at *Path, sc scope) {
	if optional || sc.direct && sc.dep.optional {
		return
	}
	if base.Kind() == types.Never {
		w.never(v, optional, at, sc)
		return
	}
	w.mismatch(v, at, sc.dep)
}

// never reports a value where Never is computed: E3801 for a required field's (TYPES.md §11.6, §13.5).
func (w *walker) never(v value.Value, optional bool, at *Path, sc scope) {
	d := sc.dep
	if !sc.direct || d.field == "" || d.optional || optional {
		w.mismatch(v, at, d)
		return
	}
	s := SiteOf(v)
	w.flag(s, w.src.related(diag.E3801.At(s.Span, d.field, w.src.types[d.app].typ), d.app), v, at)
}

// mismatch is E3802: v does not fit the type the application computes (TYPES.md §11.6).
func (w *walker) mismatch(v value.Value, at *Path, d *depSite) {
	s := SiteOf(v)
	w.flag(s, w.src.related(diag.E3802.At(s.Span, w.src.types[d.app].typ, d.whole), d.app), v, at)
}

// bareCase fills rec, the case a symbol names bare, with its defaults; a required field is E3302 (TYPES.md §8.2).
func (w *walker) bareCase(v value.Value, rec *value.Record, at *Path, d *depSite) value.Value {
	n := len(w.stage.Emitted())
	f, ok := w.stage.Bare(rec)
	w.keepSince(n, "") // a default's findings have no value path
	switch {
	case !ok:
		w.res.Poisoned, w.res.Valid = true, false
		return nil
	case f != nil:
		s := SiteOf(v)
		w.flag(s, w.src.related(diag.E3302.At(s.Span, rec.T, f.Name), d.app), v, at)
		return nil
	}
	return rec
}

func isNone(v value.Value) bool {
	_, ok := v.(*value.None)
	return ok
}
