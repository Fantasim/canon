package rules_test

import (
	"context"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/rules"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

const sharedRetagSource = `package teamboard

record Item {
  n: Int

  check self == self else "seen"
}

let w: [[Item]] = [[a, b], [c], [a, b]]
`

// EVALUATION.md §8.1: a list met again after a retag in place is walked again, per (instance, identity).
func TestSharedListWalkedAgainAfterRetag(t *testing.T) {
	fx := newFixture(t, "teamboard/item.canon", []byte(sharedRetagSource))
	seen := fx.check("check self")
	item := record("Item", field("n", types.IntType))
	item.Checks = []*syntax.CheckDecl{seen}
	mk := func(name string) *value.Record {
		return &value.Record{T: item, Fields: []value.Value{&value.Int{V: 1, T: types.IntType}}, P: fx.lit(name, "= [")}
	}
	a, b, c := mk("a"), mk("b"), mk("c")
	ev := &retagging{evaluator: fx.ev}
	var idents []bool
	fx.ev.scripts[seen] = func(self value.Value) rules.Run {
		idents = append(idents, self.(*value.Record).Ident != nil)
		if self == c {
			b.Ident = &value.Identity{Coll: &types.Collection{}, Key: value.Key{S: "b"}}
			ev.log = append(ev.log, b)
		}
		return rules.Run{}
	}
	shared := &value.List{Elems: []value.Value{a, b}}
	top := &value.List{Elems: []value.Value{shared, &value.List{Elems: []value.Value{c}}, shared}}
	r := rules.New(ev, fx.prog, map[string]*diag.Bag{pkg: fx.bag})
	if err := r.Instances(context.Background(), eval.Root{Pkg: pkg, Name: "w"}, top); err != nil {
		t.Fatal(err)
	}
	want := []string{"w[0][0]", "w[0][1]", "w[1][0]", "w[2][1]"}
	if !slices.Equal(fx.ev.paths, want) || !slices.Equal(idents, []bool{false, false, false, true}) {
		t.Errorf("runs at %q, identities %v; want at %q, b's second under its identity", fx.ev.paths, idents, want)
	}
}
