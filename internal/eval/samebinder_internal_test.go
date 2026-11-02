package eval

import (
	"testing"

	"github.com/fantasim/canonlang/internal/types"
)

// key is an argument reading binder b.
func key(b string) *types.Arg {
	return &types.Arg{Source: types.ArgKey, Binder: b}
}

// depMap is {b in c: fn(args…)}.
func depMap(b string, c *types.Collection, fn *types.TypeFunc, args ...*types.Arg) *types.DepMapType {
	return &types.DepMapType{Binder: b, Coll: c, Value: &types.TypeAppType{Fn: fn, Args: args}}
}

// EVALUATION.md §5, DECISIONS 446: two dependent maps are one type for the residue walk when they differ only in a binder's name.
func TestSameUpToBinder(t *testing.T) {
	c, d := &types.Collection{Name: "nodes"}, &types.Collection{Name: "other"}
	p := &types.TypeFunc{Name: "P", Params: []*types.Param{{Name: "e"}}}
	q := &types.TypeFunc{Name: "Q", Params: []*types.Param{{Name: "e"}, {Name: "f"}}}
	param := &types.Arg{Source: types.ArgParam, Param: q.Params[0]}
	refined := func(b string, r types.Refined) *types.DepMapType {
		d := depMap(b, c, p, key(b))
		r.Of = d.Value
		d.Value = &r
		return d
	}
	past, where := types.Refined{Past: true}, types.Refined{Where: &types.Predicate{Text: "it != none"}}
	nested := func(outer, inner, read string) *types.DepMapType {
		return &types.DepMapType{Binder: outer, Coll: c, Value: depMap(inner, d, q, key(read), key(inner))}
	}
	for _, x := range []struct {
		name string
		a    *types.DepMapType
		b    types.Type
		want bool
	}{
		{"one spelling", depMap("x", c, p, key("x")), depMap("x", c, p, key("x")), true},
		{"a binder renamed", depMap("x", c, p, key("x")), depMap("y", c, p, key("y")), true},
		{"another collection", depMap("x", c, p, key("x")), depMap("y", d, p, key("y")), false},
		{"another function", depMap("x", c, p, key("x")), depMap("x", c, q, key("x")), false},
		{"another argument count", depMap("x", c, q, key("x")), depMap("x", c, q, key("x"), key("x")), false},
		{"a parameter, not the binder", depMap("x", c, q, key("x")), depMap("x", c, q, param), false},
		{"nested binders renamed", nested("x", "y", "x"), nested("u", "v", "u"), true},
		{"a binder of another depth", nested("x", "y", "x"), nested("u", "v", "v"), false},
		{"a free binder of one name", depMap("x", c, p, key("z")), depMap("y", c, p, key("z")), true},
		{"free binders of two names", depMap("x", c, p, key("z")), depMap("y", c, p, key("w")), false},
		{"past on one side", depMap("x", c, p, key("x")), refined("y", past), false},
		{"past on both, renamed", refined("x", past), refined("y", past), true},
		{"a where on one side", depMap("x", c, p, key("x")), refined("x", where), false},
		{"another where", refined("x", where), refined("y", types.Refined{Where: &types.Predicate{Text: "it == none"}}), false},
		{"one where, renamed", refined("x", where), refined("y", where), true},
		{"not a dependent map", depMap("x", c, p, key("x")), &types.MapType{Key: types.StringType, Value: types.StringType}, false},
	} {
		if got := sameUpToBinder(x.a, x.b); got != x.want {
			t.Errorf("%s: %s and %s: got %t, want %t", x.name, x.a, x.b, got, x.want)
		}
	}
}
