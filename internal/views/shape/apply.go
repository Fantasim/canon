package shape

import (
	"slices"

	"github.com/fantasim/canonlang/internal/types"
)

// Expand is the body of a type function without a `match` applied to app's arguments: each
// argument rooted at one of its parameters is re-rooted at the application's (VIEWMODEL.md J11).
func Expand(app *types.TypeAppType) types.Type {
	return Substitute(app.Fn.Body, app.Fn.Params, app.Args)
}

// Substitute is t with every argument rooted at one of params re-rooted at the matching one of
// args, the path below the parameter kept.
func Substitute(t types.Type, params []*types.Param, args []*types.Arg) types.Type {
	re := func(x types.Type) types.Type { return Substitute(x, params, args) }
	switch x := t.(type) {
	case *types.TypeAppType:
		return &types.TypeAppType{Fn: x.Fn, Args: reroot(x.Args, params, args)}
	case *types.AppliedRecord:
		return &types.AppliedRecord{Rec: x.Rec, Args: reroot(x.Args, params, args)}
	case *types.OptionalType:
		return &types.OptionalType{Elem: re(x.Elem)}
	case *types.ListType:
		return &types.ListType{Elem: re(x.Elem), KeyedBy: x.KeyedBy}
	case *types.MapType:
		return &types.MapType{Key: re(x.Key), Value: re(x.Value)}
	case *types.DepMapType:
		return &types.DepMapType{Binder: x.Binder, Coll: x.Coll, Value: re(x.Value)}
	case *types.LitUnionType:
		return &types.LitUnionType{Of: re(x.Of), Literals: x.Literals}
	case *types.Refined:
		r := *x
		r.Of = re(x.Of)
		return &r
	}
	return t
}

// reroot replaces each argument rooted at a parameter of params by the matching argument of
// args, followed by the rest of its path.
func reroot(in []*types.Arg, params []*types.Param, args []*types.Arg) []*types.Arg {
	out := make([]*types.Arg, len(in))
	for i, a := range in {
		out[i] = a
		if a.Source != types.ArgParam || !slices.Contains(params, a.Param) || a.Param.Index >= len(args) {
			continue
		}
		outer := *args[a.Param.Index]
		outer.Path = slices.Concat(outer.Path, a.Path)
		out[i] = &outer
	}
	return out
}
