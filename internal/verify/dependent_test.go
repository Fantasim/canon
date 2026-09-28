package verify_test

import (
	"context"
	"errors"
	"testing"

	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/verify"
)

// eventSource is `type P(e: Ev) = match e.k { one => Int, two => String }` given to `p: P(ev)`.
const eventSource = `package teamboard

let evs: table Ev = { a { k: one }, b { k: two } }

let t: Thing = { ev: a, p: 1 }

let u: Thing = { ev: b, p: pick }
`

// events builds eventSource's types and values: evs, then t and u, whose p is 1 and `pick`;
// without args, P is applied to none of its arguments.
func events(fx *fixture, args bool) {
	k := &types.EnumType{Pkg: pkg, Name: "K", Members: []*types.Member{{Name: "one"}, {Name: "two", Index: 1}}}
	ev := record("Ev", field("k", k))
	coll := collection("evs", ev)
	evs := &types.TableType{Elem: ev}
	e := &types.Param{Name: "e", Type: ev}
	p := &types.TypeFunc{Pkg: pkg, Name: "P", Params: []*types.Param{e},
		Scrutinee: &types.Scrutinee{Param: e, Path: []*types.Field{ev.Fields[0]}, Type: k},
		Arms:      []*types.TypeArm{{Members: []int{0}, Result: types.IntType}, {Members: []int{1}, Result: types.StringType}},
	}
	evRef := &types.RefType{Target: coll}
	evField := field("ev", evRef)
	app := &types.TypeAppType{Fn: p}
	if args {
		app.Args = []*types.Arg{{Source: types.ArgField, Path: []*types.Field{evField}}}
	}
	thing := record("Thing", evField, field("p", app))
	entry := func(key string, m int) *value.Record {
		return fx.entry(coll, key, ev, &value.Member{Enum: k, Index: m, P: fx.lit("k: ", key+" {")})
	}
	fx.let("evs", evs, &value.Table{T: evs, Entries: []*value.Record{entry("a", 0), entry("b", 1)}, P: fx.lit("{", "let evs")})
	thingOf := func(key string, pv value.Value, within string) *value.Record {
		ref := &value.Ref{T: evRef, Key: value.Key{S: key}, P: fx.lit(key, within)}
		return &value.Record{T: thing, Fields: []value.Value{ref, pv}, Set: []bool{true, true}, P: fx.lit("{", within)}
	}
	fx.let("t", thing, thingOf("a", integer(1, types.IntType, fx.lit("1", "let t")), "let t"))
	fx.let("u", thing, thingOf("b", &value.Symbol{Name: "pick", T: thing.Fields[1].Type, P: fx.lit("pick")}, "let u"))
}

// TYPES.md §11.6: without an evaluator of dependent types, or with a wrong arity, verification stops.
func TestUnjudgedStops(t *testing.T) {
	for _, args := range []bool{true, false} {
		fx := newFixture(t, "teamboard/events.canon", []byte(eventSource))
		events(fx, args)
		root := eval.Root{Pkg: pkg, Name: "u"}
		_, err := fx.verifier().Check(context.Background(), root, fx.ev.values[root])
		sym := fx.ev.values[root].(*value.Record).Fields[1]
		if _, kept := sym.(*value.Symbol); !errors.Is(err, verify.ErrUnjudged) || !kept || len(fx.bag.Findings()) != 0 {
			t.Errorf("args %t: err %v, p %T, findings %v; want ErrUnjudged, the symbol untouched, none", args, err, sym, codes(fx))
		}
	}
}
