package types_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/types"
)

// TYPES.md §11.5, DECISIONS 338: only a dependent map's own keys bind its binder.
func TestMapBinder(t *testing.T) {
	keyArg := []*types.Arg{{Source: types.ArgKey, Binder: "x"}}
	value := &types.AppliedRecord{Rec: status, Args: keyArg}
	dep := &types.DepMapType{Binder: "x", Coll: statuses, Value: value}
	nested := &types.DepMapType{Binder: "x", Coll: statuses, Value: &types.MapType{Key: refOther, Value: value}}
	view := &types.MapType{Key: refStatus, Value: value}
	inner := &types.MapType{Key: refOther, Value: value}
	byName := &types.MapType{Key: types.StringType, Value: value}
	none := func(string) bool { return false }
	bound := func(name string) bool { return name == "x" }
	for _, c := range []struct {
		name        string
		t, declared types.Type
		bound       func(string) bool
		want        string
	}{
		{"a dependent map binds its own", dep, nil, none, "x"},
		{"a static view binds its declared dependent map's", view, dep, none, "x"},
		{"a static view with no declared one binds none", view, nil, none, ""},
		{"a static view whose declared type has none binds none", view, types.BoolType, none, ""},
		{"a String-keyed map never binds", byName, dep, none, ""},
		{"a map keyed by another collection binds none", inner, nested, bound, ""},
		{"a map on the same collection keeps a bound binder", view, dep, bound, ""},
	} {
		if got := types.MapBinder(c.t, c.declared, c.bound); got != c.want {
			t.Errorf("%s: MapBinder = %q, want %q", c.name, got, c.want)
		}
	}
}

// TYPES.md §11.5, DECISIONS 408: kept when the value reads the binder and every argument is a bare binder.
func TestKeptDepMap(t *testing.T) {
	key := func(b string) *types.Arg { return &types.Arg{Source: types.ArgKey, Binder: b} }
	app := func(args ...*types.Arg) types.Type { return &types.AppliedRecord{Rec: status, Args: args} }
	dep := func(b string, v types.Type) *types.DepMapType {
		return &types.DepMapType{Binder: b, Coll: statuses, Value: v}
	}
	field := &types.Arg{Source: types.ArgField, Path: []*types.Field{{Name: "like"}}}
	param := &types.Arg{Source: types.ArgParam, Param: &types.Param{Name: "e"}}
	path := &types.Arg{Source: types.ArgKey, Binder: "x", Path: []*types.Field{{Name: "next"}}}
	for _, c := range []struct {
		name string
		d    *types.DepMapType
		want bool
	}{
		{"its binder", dep("x", app(key("x"))), true},
		{"through an optional, a list, a map value and a pair", dep("x", &types.OptionalType{Elem: list(&types.MapType{Key: types.StringType, Value: &types.PairType{A: types.IntType, B: app(key("x"))}})}), true},
		{"through a literal union over an application", dep("x", &types.LitUnionType{Of: &types.TypeAppType{Fn: paramFn, Args: []*types.Arg{key("x")}}, Literals: []string{"none"}}), true},
		{"its binder and an inner map's", dep("x", dep("y", app(key("x"), key("y")))), true},
		{"a value that reads no binder", dep("x", types.IntType), false},
		{"an earlier field too", dep("x", app(key("x"), field)), false},
		{"a parameter too", dep("x", app(key("x"), param)), false},
		{"a path from its binder", dep("x", app(path)), false},
		{"an outer binder only", dep("x", app(key("z"))), false},
		{"only an inner map's binder", dep("x", dep("y", app(key("y")))), false},
		{"its binder shadowed by an inner map's", dep("x", dep("x", app(key("x")))), false},
		{"a dependent key reading a field", dep("x", &types.MapType{Key: &types.TypeAppType{Fn: paramFn, Args: []*types.Arg{field}}, Value: app(key("x"))}), false},
	} {
		if got := types.KeptDepMap(c.d); got != c.want {
			t.Errorf("%s: KeptDepMap(%s) = %v, want %v", c.name, c.d, got, c.want)
		}
	}
}

// TYPES.md §11.4: a parameter is read along the declared type's first application reaching it.
func TestChainArg(t *testing.T) {
	e, n := &types.Param{Name: "e"}, &types.Param{Name: "n"}
	inner := &types.TypeFunc{Name: "Q", Params: []*types.Param{n}, Body: types.IntType}
	path := []*types.Field{{Name: "next"}}
	outer := &types.TypeFunc{Name: "P", Params: []*types.Param{e}}
	outer.Body = &types.TypeAppType{Fn: inner, Args: []*types.Arg{{Source: types.ArgParam, Param: e, Path: path}}}
	root := &types.Arg{Source: types.ArgField, Path: []*types.Field{{Name: "like"}}}
	declared := &types.ListType{Elem: &types.TypeAppType{Fn: outer, Args: []*types.Arg{root}}}
	if a, paths, ok := types.ChainArg(declared, n); !ok || a != root || len(paths) != 1 || paths[0][0] != path[0] {
		t.Errorf("ChainArg(n) = %v, %v, %v; want the field like, then next", a, paths, ok)
	}
	if a, paths, ok := types.ChainArg(declared, e); !ok || a != root || len(paths) != 0 {
		t.Errorf("ChainArg(e) = %v, %v, %v; want the field like", a, paths, ok)
	}
	if _, _, ok := types.ChainArg(types.IntType, e); ok {
		t.Error("ChainArg on a type with no application reached p")
	}
}
