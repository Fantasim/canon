package check

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
)

// pastType is r, or `past r` when t is written `past …`; r kept on E3024 or an error (TYPES.md §8.4).
func (c *checker) pastType(tc *typeCtx, t syntax.Type, r types.Type) types.Type {
	if syntax.PastOf(t) == syntax.NoTok || r.Kind() == types.Error {
		return r
	}
	p, ok := types.Past(r)
	if !ok {
		c.report(tc.env, diag.E3024.At(tc.env.span(t), r))
	}
	return p
}

// inferred is t with every `past` layer removed: `past` comes only from a written type (TYPES.md §8.4).
func inferred(t types.Type) types.Type {
	if t == nil {
		return nil
	}
	u, _ := unpast(t)
	return u
}

// unpast is inferred's walk: t rebuilt where a part held a `past`, and whether one did.
func unpast(t types.Type) (types.Type, bool) {
	switch x := t.(type) {
	case *types.Refined:
		return unpastRefined(x)
	case *types.Alias:
		if d, changed := unpast(x.Def); changed {
			return d, true
		}
	case *types.OptionalType:
		return unpastOne(t, x.Elem, func(e types.Type) types.Type { return &types.OptionalType{Elem: e} })
	case *types.ListType:
		return unpastOne(t, x.Elem, func(e types.Type) types.Type { return &types.ListType{Elem: e, KeyedBy: x.KeyedBy} })
	case *types.LitUnionType:
		return unpastOne(t, x.Of, func(e types.Type) types.Type { return &types.LitUnionType{Of: e, Literals: x.Literals} })
	case *types.DepMapType:
		return unpastOne(t, x.Value, func(e types.Type) types.Type {
			return &types.DepMapType{Binder: x.Binder, Coll: x.Coll, Value: e}
		})
	case *types.MapType:
		return unpastParts(t, []types.Type{x.Key, x.Value}, func(p []types.Type) types.Type { return &types.MapType{Key: p[0], Value: p[1]} })
	case *types.PairType:
		return unpastParts(t, []types.Type{x.A, x.B}, func(p []types.Type) types.Type { return &types.PairType{A: p[0], B: p[1]} })
	case *types.FuncType:
		return unpastParts(t, append([]types.Type{x.Result}, x.Params...), func(p []types.Type) types.Type {
			return &types.FuncType{Result: p[0], Params: p[1:]}
		})
	}
	return t, false
}

// unpastOne is unpast on a type of one part, rebuilt by build when the part held a `past`.
func unpastOne(t, part types.Type, build func(types.Type) types.Type) (types.Type, bool) {
	u, changed := unpast(part)
	if !changed {
		return t, false
	}
	return build(u), true
}

// unpastParts is unpast on a type of several parts, rebuilt by build when one held a `past`.
func unpastParts(t types.Type, parts []types.Type, build func([]types.Type) types.Type) (types.Type, bool) {
	out := make([]types.Type, len(parts))
	held := false
	for i, p := range parts {
		var changed bool
		if p == nil {
			out[i] = nil
			continue
		}
		out[i], changed = unpast(p)
		held = held || changed
	}
	if !held {
		return t, false
	}
	return build(out), true
}

// unpastRefined clears a `past` flag, keeping the node while another refinement remains on it.
func unpastRefined(r *types.Refined) (types.Type, bool) {
	of, changed := unpast(r.Of)
	if !r.Past && !changed {
		return r, false
	}
	cp := *r
	cp.Of, cp.Past = of, false
	if types.PastOnly(&cp) {
		return of, true
	}
	return &cp, true
}
