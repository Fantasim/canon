package rules_test

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/rules"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// The shared-nesting scale case of stage C: the tries per depth, and the growth in traversal time
// allowed from the smallest depth to the largest. Each level doubles the paths to the leaf, so a
// walk that re-visits a shared value grows by 2^10 over the table.
const (
	sharedTries  = 3
	sharedGrowth = 16
)

// sharedDepths are the depths the scale case builds, smallest first.
var sharedDepths = []int{10, 15, 20}

const sharedLeafSource = `package teamboard

record Leaf {
  n: Int

  check n > 1 else "small"
}

let w: [[Leaf]] = [[{ n: 1 }]]
`

// sharedCase is a chain of lists, each level holding the level below twice, over one Leaf
// instance whose check fails: its fixture, the chain's top and declared type.
type sharedCase struct {
	fx   *fixture
	leaf *syntax.CheckDecl
	top  value.Value
	dt   types.Type
}

func newSharedCase(t *testing.T, depth int) *sharedCase {
	fx := newFixture(t, "teamboard/leaf.canon", []byte(sharedLeafSource))
	sc := &sharedCase{fx: fx, leaf: fx.check("check n")}
	leafT := record("Leaf", field("n", types.IntType))
	leafT.Checks = []*syntax.CheckDecl{sc.leaf}
	fx.ev.scripts[sc.leaf] = fails("small")
	one := &value.Int{V: 1, T: types.IntType, P: fx.lit("1", "{ n:")}
	leaf := &value.Record{T: leafT, Fields: []value.Value{one}, P: fx.lit("{ n:")}
	var dt types.Type = &types.ListType{Elem: leafT}
	var top value.Value = &value.List{T: dt, Elems: []value.Value{leaf}, P: fx.lit("[{")}
	for range depth {
		dt = &types.ListType{Elem: dt}
		top = &value.List{T: dt, Elems: []value.Value{top, top}, P: fx.lit("[[")}
	}
	sc.top, sc.dt = top, dt
	return sc
}

// instances is stage C over the case's top, and the time it took.
func (sc *sharedCase) instances(t *testing.T) time.Duration {
	r := rules.New(sc.fx.ev, sc.fx.prog, map[string]*diag.Bag{pkg: sc.fx.bag})
	start := time.Now()
	err := r.Instances(context.Background(), eval.Root{Pkg: pkg, Name: "w"}, sc.top)
	took := time.Since(start)
	if err != nil {
		t.Fatal(err)
	}
	return took
}

// sharedFastest is the least of sharedTries traversals' times at depth.
func sharedFastest(t *testing.T, depth int) time.Duration {
	best := time.Duration(1<<63 - 1)
	for range sharedTries {
		best = min(best, newSharedCase(t, depth).instances(t))
	}
	return best
}

// EVALUATION.md §8.1, DECISIONS 507: stage C is linear in depth; a wall-clock test, its margin wide (16x).
func TestSharedNestingScale(t *testing.T) {
	small := sharedFastest(t, sharedDepths[0])
	for _, d := range sharedDepths[1:] {
		if took := sharedFastest(t, d); took > sharedGrowth*max(small, time.Millisecond) {
			t.Errorf("depth %d: %v, depth %d: %v, more than %dx", sharedDepths[0], small, d, took, sharedGrowth)
		}
	}
}

// EVALUATION.md §8.1, §8.3, DECISIONS 504, 507: the shared leaf is checked once, at its first place.
func TestSharedNestingFirstPlace(t *testing.T) {
	const depth = 4
	sc := newSharedCase(t, depth)
	sc.instances(t)
	want := strings.Repeat("[0]", depth+1)
	if !slices.Equal(sc.fx.ev.calls, []*syntax.CheckDecl{sc.leaf}) || !slices.Equal(sc.fx.ev.paths, []string{"w" + want}) {
		t.Errorf("runs %v at %q, want one at w%s", sc.fx.ev.calls, sc.fx.ev.paths, want)
	}
	got := sc.fx.bag.Findings()
	if len(got) != 1 || got[0].Path != "w"+want {
		t.Errorf("findings %+v, want one at w%s", got, want)
	}
}
