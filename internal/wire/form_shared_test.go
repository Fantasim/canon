package wire_test

import (
	"slices"
	"testing"
	"time"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/wire"
)

// The shared-nesting scale case of FieldForms: the tries per depth, and the growth in walk time
// allowed from the smallest depth to the largest. Each level doubles the paths to the leaf, so a
// walk that re-visits a shared value grows by 2^10 over the depths.
const (
	sharedTries  = 3
	sharedGrowth = 16
)

// sharedDepths are the depths the scale case builds, smallest first.
var sharedDepths = []int{10, 15, 20}

// sharedChain is a chain of lists over leaf, each level holding the level below twice.
func sharedChain(depth int, leaf value.Value) value.Value {
	var t types.Type = types.DurationType
	top := leaf
	for range depth + 1 {
		top = &value.List{T: listOf(t), Elems: []value.Value{top, top}}
		t = listOf(t)
	}
	return top
}

// sharedFastest is the least of sharedTries FieldForms walks' times over a chain of depth.
func sharedFastest(t *testing.T, depth int) time.Duration {
	v := sharedChain(depth, dur(1000))
	f := unitField(v.Type(), types.UnitS)
	best := time.Duration(1<<63 - 1)
	for range sharedTries {
		start := time.Now()
		forms := wire.FieldForms(f, v)
		best = min(best, time.Since(start))
		if len(forms) != 0 {
			t.Fatalf("depth %d: %d parts, want none", depth, len(forms))
		}
	}
	return best
}

// WIRE.md §5.1, DECISIONS 511: FieldForms walks a shared value once; a wall-clock test, its margin wide (16x).
func TestFieldFormsSharedScale(t *testing.T) {
	small := sharedFastest(t, sharedDepths[0])
	for _, d := range sharedDepths[1:] {
		if took := sharedFastest(t, d); took > sharedGrowth*max(small, time.Millisecond) {
			t.Errorf("depth %d: %v, depth %d: %v, more than %dx", sharedDepths[0], small, d, took, sharedGrowth)
		}
	}
}

// WIRE.md §5.1, DECISIONS 511: a shared part is listed once, at its first place in walk order ([1] before [10]).
func TestFieldFormsSharedFirstPlace(t *testing.T) {
	odd := dur(1500)
	bad := list(types.DurationType, dur(1000), odd)
	elems := make([]value.Value, sharedDepths[0]+1)
	for i := range elems {
		elems[i] = list(types.DurationType, dur(1000))
	}
	elems[1], elems[len(elems)-1] = bad, bad
	wide := &value.List{T: listOf(listOf(types.DurationType)), Elems: elems}
	twice := list(types.DurationType, dur(2000), odd, odd)
	byName := &types.MapType{Key: types.StringType, Value: listOf(types.DurationType)}
	m := &value.Map{T: byName, Keys: []value.Value{str("z"), str("a")}, Vals: []value.Value{bad, bad}}
	chain := sharedChain(sharedDepths[0], odd)
	for _, c := range []struct {
		name string
		v    value.Value
		want []wire.Step
	}{
		{"a list at [1] and [10]", wide, []wire.Step{{In: wide, Index: 1}, {In: bad, Index: 1}}},
		{"a Duration twice", twice, []wire.Step{{In: twice, Index: 1}}},
		{"a list under two keys", m, []wire.Step{{In: m, Key: m.Keys[0]}, {In: bad, Index: 1}}},
		{"a chain", chain, leftmost(chain)},
	} {
		forms := wire.FieldForms(unitField(c.v.Type(), types.UnitS), c.v)
		if len(forms) != 1 || forms[0].Value != odd || !slices.Equal(forms[0].Steps, c.want) {
			t.Errorf("%s: %+v, want one part, %v at %+v", c.name, forms, odd, c.want)
		}
	}
}

// leftmost is the steps from v down its first elements to the first value that is not a list.
func leftmost(v value.Value) []wire.Step {
	var steps []wire.Step
	for l, ok := v.(*value.List); ok; l, ok = l.Elems[0].(*value.List) {
		steps = append(steps, wire.Step{In: l})
	}
	return steps
}
