package eval

import (
	"context"
	"math"
	"testing"

	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// eqDepth is how deep eqChain nests, past the 1024 pairs after which both walks remember pairs;
// kvStride is a key and its value.
const (
	eqDepth  = 12
	kvStride = 2
)

// eqChain is a list of two copies of the one below, depth times: a DAG of 2^depth paths.
func eqChain(depth int) value.Value {
	var v value.Value = &value.Int{V: 1, T: types.IntType}
	lt := &types.ListType{Elem: types.IntType}
	for range depth {
		v = &value.List{T: lt, Elems: []value.Value{v, v}}
	}
	return v
}

// TYPES.md §7.5, DECISIONS 197: without symbols, the result and step count of value.EqualUpTo, memo included.
func TestEqualMatchesValue(t *testing.T) {
	it := func(n int64) value.Value { return &value.Int{V: n, T: types.IntType} }
	str := func(s string) value.Value { return &value.Str{V: s, T: types.StringType} }
	lt := &types.ListType{Elem: types.IntType}
	list := func(vs ...value.Value) value.Value { return &value.List{T: lt, Elems: vs} }
	mt := &types.MapType{Key: types.StringType, Value: types.IntType}
	mp := func(kv ...value.Value) value.Value {
		m := &value.Map{T: mt}
		for i := 0; i+1 < len(kv); i += kvStride {
			m.Keys, m.Vals = append(m.Keys, kv[i]), append(m.Vals, kv[i+1])
		}
		return m
	}
	rt := &types.RecordType{Pkg: "a", Name: "R", Fields: []*types.Field{{Name: "n", Type: types.IntType}, {Name: "s", Index: 1, Type: types.StringType}}}
	coll := &types.Collection{Kind: types.CollLet, Pkg: "a", Name: "rs", Elem: rt}
	rec := func(key string, n int64, s string) *value.Record {
		r := &value.Record{T: rt, Fields: []value.Value{it(n), str(s)}}
		if key != "" {
			r.Ident = &value.Identity{Coll: coll, Key: value.Key{S: key}}
		}
		return r
	}
	tt := &types.TableType{Elem: rt}
	table := func(es ...*value.Record) value.Value { return &value.Table{T: tt, Entries: es} }
	ref := func(key string) value.Value {
		return &value.Ref{T: &types.RefType{Target: coll}, Key: value.Key{S: key}}
	}
	cases := []struct {
		name string
		a, b value.Value
	}{
		{"scalars", it(1), it(1)},
		{"unequal scalars", str("a"), str("b")},
		{"floats", &value.Float{V: math.Copysign(0, -1), T: types.FloatType}, &value.Float{T: types.FloatType}},
		{"lists", list(it(1), list(it(2), it(3))), list(it(1), list(it(2), it(3)))},
		{"lists differing deep", list(it(1), list(it(2), it(3))), list(it(1), list(it(2), it(4)))},
		{"lists of two lengths", list(it(1)), list(it(1), it(2))},
		{"maps in two orders", mp(str("a"), it(1), str("b"), it(2)), mp(str("b"), it(2), str("a"), it(1))},
		{"maps differing", mp(str("a"), it(1)), mp(str("a"), it(2))},
		{"records", rec("", 1, "x"), rec("", 1, "x")},
		{"entries by identity", rec("k", 1, "x"), rec("k", 2, "y")},
		{"an entry and its ref", rec("k", 1, "x"), ref("k")},
		{"refs", ref("k"), ref("j")},
		{"tables field-wise", table(rec("k", 1, "x")), table(rec("k", 1, "x"))},
		{"tables differing in an entry", table(rec("k", 1, "x")), table(rec("k", 1, "y"))},
		{"tables of other keys", table(rec("k", 1, "x")), table(rec("j", 1, "x"))},
		{"pairs", &value.Pair{A: it(1), B: str("a")}, &value.Pair{A: it(1), B: str("a")}},
		{"a DAG past the memo", eqChain(eqDepth), eqChain(eqDepth)},
	}
	for _, c := range cases {
		e := newEvaluator(nil, Options{Budget: math.MaxInt64})
		r := e.newRun(context.Background(), charge{}, nil)
		eq, ok := r.equal(c.a, c.b, func() source.Span { return source.Span{} })
		weq, wok, n := value.EqualUpTo(c.a, c.b, math.MaxInt)
		if eq != weq || ok != wok || e.steps != int64(n) {
			t.Errorf("%s: %t %t in %d steps, value.EqualUpTo %t %t in %d", c.name, eq, ok, e.steps, weq, wok, n)
		}
	}
}
