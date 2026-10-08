package check

import (
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// bindCtx is where evaluation reads a record literal's type arguments (TYPES.md §11.1, §11.5); nil: nowhere.
type bindCtx struct {
	field   types.Type
	rec     bool
	params  []*types.Param
	binders []string
}

// forField is b giving a field of declared type t its value.
func (b *bindCtx) forField(t types.Type) *bindCtx {
	out := bindCtx{field: t}
	if b != nil {
		out = *b
		out.field = t
	}
	return &out
}

// withBinder is b with a dependent map's binder bound; b itself for none.
func (b *bindCtx) withBinder(name string) *bindCtx {
	if name == "" {
		return b
	}
	out := bindCtx{}
	if b != nil {
		out = *b
	}
	out.binders = append(slices.Clip(out.binders), name)
	return &out
}

// step is b past an amendment segment of container t to type next, a record field giving its record's (EVALUATION.md §9.2).
func (b *bindCtx) step(t types.Type, seg *syntax.AmendSegment, next types.Type) *bindCtx {
	switch t.Base().(type) {
	case *types.RecordType, *types.AppliedRecord, *types.CaseType, *types.VariantType:
		if seg.Name != nil {
			return inRecord(t).forField(next)
		}
	}
	return b
}

// inRecord is the context of the fields of a record literal of type t (its parameters bound).
func inRecord(t types.Type) *bindCtx {
	out := &bindCtx{rec: true}
	switch x := t.Base().(type) {
	case *types.AppliedRecord:
		out.params = x.Rec.Params
	case *types.RecordType:
		out.params = x.Params
	}
	return out
}

// applied reports a record literal of type t whose arguments b reads; true for another type.
func (b *bindCtx) applied(t types.Type) bool {
	app, ok := t.Base().(*types.AppliedRecord)
	return !ok || !slices.ContainsFunc(app.Args, func(a *types.Arg) bool { return !b.arg(a) })
}

// arg reports a type argument b reads: a bound parameter, one a declared application reaches,
// a bound binder or a field of the record being built.
func (b *bindCtx) arg(a *types.Arg) bool {
	switch {
	case b == nil:
		return false
	case a.Source == types.ArgParam && slices.Contains(b.params, a.Param):
		return true
	case a.Source == types.ArgParam:
		root, _, ok := types.ChainArg(b.field, a.Param)
		return ok && b.arg(root)
	case a.Source == types.ArgKey:
		return slices.Contains(b.binders, a.Binder)
	}
	return len(a.Path) > 0 && b.rec
}

// mapBinder is the binder a map literal of type t binds to each key here, as evaluation does.
func (b *bindCtx) mapBinder(t types.Type) string {
	var declared types.Type
	if b != nil {
		declared = b.field
	}
	return types.MapBinder(t, declared, func(name string) bool { return b != nil && slices.Contains(b.binders, name) })
}

// unboundLiteral is E3804 for a record literal whose arguments nothing binds here (TYPES.md §11.4).
func (c *checker) unboundLiteral(env *env, e *syntax.BraceLit, t types.Type) bool {
	if env.bind.applied(t) {
		return false
	}
	c.report(env, diag.E3804.At(env.span(e), opLiteral, t.Base().(*types.AppliedRecord).Rec.Name))
	c.info.Literals[e] = LitError
	c.itemsInError(env, e.Items)
	return true
}
