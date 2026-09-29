package rules_test

import (
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/rules"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

const askedSource = `package teamboard

record Leaf {
  size: Int

  check size > 0 else "never"
}

record Holder {
  v: Int

  check v > 100 else "small"
}

record Duo {
  a: Leaf
  b: Holder
}
`

// marking is the fixture evaluator telling its marks, as eval's does: stage C then keeps no
// answer of its invalid test until a check run marks a value.
type marking struct {
	*evaluator
}

func (m marking) InvalidCount() int { return len(m.invalid) }

func (m marking) InvalidValues() []value.Value {
	var out []value.Value
	for v := range m.invalid { //canon:unordered a set
		out = append(out, v)
	}
	return out
}

// askedCase is the values of askedSource: duo holds s under a Leaf, whose check marks s and u,
// then under a Holder; asked and later hold s, fresh holds u, first asked after the mark.
func askedCase(t *testing.T, markedFirst bool) *fixture {
	fx := newFixture(t, "teamboard/asked.canon", []byte(askedSource))
	leaf := record("Leaf", field("size", types.IntType))
	holder := record("Holder", field("v", types.IntType))
	duo := record("Duo", field("a", leaf), field("b", holder))
	leaf.Checks = append(leaf.Checks, fx.check("check size"))
	holder.Checks = append(holder.Checks, fx.check("check v"))
	s := &value.Int{V: -1, T: types.IntType, P: fx.lit("Int", "size")}
	u := &value.Int{V: -2, T: types.IntType, P: fx.lit("Int", "v")}
	fx.ev.scripts[leaf.Checks[0]] = func(value.Value) rules.Run {
		fx.ev.invalid[s], fx.ev.invalid[u] = true, true
		return rules.Run{}
	}
	fx.ev.scripts[holder.Checks[0]] = fails("small")
	if markedFirst {
		fx.ev.invalid[&value.Int{V: 0, T: types.IntType}] = true
	}
	hold := func(v value.Value, at string) *value.Record { // each where findings on it are located
		return &value.Record{T: holder, Fields: []value.Value{v}, P: fx.lit(at)}
	}
	fx.let("duo", &value.Record{T: duo, Fields: []value.Value{&value.Record{T: leaf, Fields: []value.Value{s}, P: fx.lit("Leaf")}, hold(s, "b: Holder")}, P: fx.lit("Duo")})
	fx.let("asked", hold(s, "record Holder"))
	fx.let("fresh", hold(u, "v: Int"))
	return fx
}

// EVALUATION.md §7.3: each answer is kept from its first asking, marks told or not, set or not.
func TestInvalidAskedFirst(t *testing.T) {
	for _, markedFirst := range []bool{false, true} {
		eager := askedCase(t, markedFirst)
		eager.run()
		told := askedCase(t, markedFirst)
		told.runWith(marking{told.ev})
		if got, want := string(told.render()), string(eager.render()); got != want {
			t.Errorf("marked first %t: marks told:\n%s\nnot told:\n%s", markedFirst, got, want)
		}
		if paths := failedPaths(told); !slices.Equal(paths, []string{"asked", "duo.b"}) {
			t.Errorf("marked first %t: failed checks at %v, want asked and duo.b", markedFirst, paths)
		}
	}
}

// failedPaths is the paths of the failed one-line check findings of fx, in order.
func failedPaths(fx *fixture) []string {
	var out []string
	for _, f := range fx.bag.Findings() {
		if f.Code == diag.E5001.Def().Code {
			out = append(out, f.Path)
		}
	}
	return out
}
