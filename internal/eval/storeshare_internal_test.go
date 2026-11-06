package eval

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// storeDepth is how deep storeChain nests: 2^storeDepth paths reach its leaf. A binary32 value
// stays as it is stored; an unrounded one is rounded, and is past belowUnrounded's bound.
const (
	storeDepth     = 20
	binary32       = 1.5
	unrounded      = 0.1
	belowUnrounded = 0.05
	storeTries     = 3
	storeGrows     = 16
)

// storeDepths are the depths the scale case stores, smallest first: each level doubles the paths.
var storeDepths = []int{10, 15, 20}

// storeLeafCase is a leaf with a storage check and its declared type.
type storeLeafCase struct {
	name string
	leaf value.Value
	t    types.Type
}

// storeLeafCases are the leaves of the scale case: the Float32 ones kept and rebuilt.
func storeLeafCases() []storeLeafCase {
	natural := &types.Refined{Of: types.IntType, Range: &types.Bound{HasLo: true}}
	return []storeLeafCase{
		{"duration", &value.Dur{Ms: 1}, types.DurationType},
		{"int8", &value.Int{V: 1, T: types.Int8Type}, types.Int8Type},
		{"float32", &value.Float{V: binary32, T: types.Float32Type}, types.Float32Type},
		{"float32/rounded", &value.Float{V: unrounded, T: types.FloatType}, types.Float32Type},
		{"refined", &value.Int{V: 1, T: types.IntType}, natural},
	}
}

// storeChain is w_0 = [leaf], w_i = [w_{i-1}, w_{i-1}] to depth, and its type, a list of elem at each level.
func storeChain(leaf value.Value, elem types.Type, depth int) (value.Value, types.Type) {
	t := types.Type(&types.ListType{Elem: elem})
	v := value.Value(&value.List{T: t, Elems: []value.Value{leaf}})
	for range depth {
		t = &types.ListType{Elem: t}
		v = &value.List{T: t, Elems: []value.Value{v, v}}
	}
	return v, t
}

// storeRun is a run of an evaluator of no program.
func storeRun() *run {
	return &run{ev: newEvaluator(nil, Options{}), ctx: context.Background()}
}

// EVALUATION.md §4.3; DECISIONS 513: a shared tree of rounded Float32s is stored as the same pointer.
func TestStoreKeepsFloat32Sharing(t *testing.T) {
	leaf := &value.Float{V: binary32, T: types.Float32Type}
	v, lt := storeChain(leaf, types.Float32Type, storeDepth)
	if got := storeRun().store(v, lt, site{}, nil); got != v {
		t.Errorf("store gave another value, want the same pointer")
	}
}

// EVALUATION.md §4.3; DECISIONS 513: a shared value rebuilt by its conversion is rebuilt once, still shared.
func TestStoreRebuildsSharedOnce(t *testing.T) {
	leaf := &value.Float{V: unrounded, T: types.FloatType}
	v, lt := storeChain(leaf, types.Float32Type, storeDepth)
	got := storeRun().store(v, lt, site{}, nil)
	for level := storeDepth; level > 0; level-- {
		l, ok := got.(*value.List)
		if !ok || len(l.Elems) != len(v.(*value.List).Elems) || l.Elems[0] != l.Elems[1] {
			t.Fatalf("level %d: %T, want a list of one element twice", level, got)
		}
		got, v = l.Elems[0], v.(*value.List).Elems[0]
	}
	l, ok := got.(*value.List)
	if !ok || len(l.Elems) != 1 {
		t.Fatalf("innermost: %T, want a list of one", got)
	}
	f, ok := l.Elems[0].(*value.Float)
	if !ok || f.V != float64(float32(leaf.V)) || f.T != types.Type(types.Float32Type) {
		t.Errorf("leaf %v, want %v rounded to Float32", l.Elems[0], leaf.V)
	}
}

// EVALUATION.md §7.3; DECISIONS 513: a container converted again after a value was marked carries the mark.
func TestStoreRewalksAfterMark(t *testing.T) {
	// Each pair converts a as [Float32], rounding it, then as [Float(..0.05)], marking its element:
	// the second pair's A, met after that mark, is rebuilt from the marked element.
	small := &types.Refined{Of: types.FloatType, Range: &types.Bound{HasHi: true, Hi: types.Limit{F: belowUnrounded}}}
	pt := &types.PairType{A: &types.ListType{Elem: types.Float32Type}, B: &types.ListType{Elem: small}}
	lt := &types.ListType{Elem: pt}
	a := &value.List{T: &types.ListType{Elem: types.FloatType}, Elems: []value.Value{&value.Float{V: unrounded, T: types.FloatType}}}
	v := &value.List{T: lt, Elems: []value.Value{&value.Pair{T: pt, A: a, B: a}, &value.Pair{T: pt, A: a, B: a}}}
	r := storeRun()
	r.fr = &frame{}
	got, ok := r.store(v, lt, site{}, nil).(*value.List)
	if !ok || len(got.Elems) != len(v.Elems) {
		t.Fatalf("store gave %T, want a list of two pairs", got)
	}
	second, ok := got.Elems[1].(*value.Pair)
	if !ok {
		t.Fatalf("second element %T, want a pair", got.Elems[1])
	}
	if l, isList := second.A.(*value.List); !isList || len(l.Elems) != 1 || !r.ev.Invalid(l.Elems[0]) {
		t.Errorf("the second pair's rounded element lost the invalid mark")
	}
}

// DECISIONS 513: a map no key or value of which changes is kept, as a list is.
func TestStoreKeepsUnchangedMap(t *testing.T) {
	mt := &types.MapType{Key: types.StringType, Value: types.Float32Type}
	m := &value.Map{T: mt, Keys: []value.Value{&value.Str{V: "a", T: types.StringType}}, Vals: []value.Value{&value.Float{V: binary32, T: types.Float32Type}}}
	if got := storeRun().store(m, mt, site{}, nil); got != m {
		t.Errorf("store rebuilt a map it changed nothing in")
	}
}

// storeTime is the least of storeTries times taken to store c's chain at depth as a let does, or
// in the two fields of a record, each field a conversion of its own.
func storeTime(t *testing.T, c storeLeafCase, depth int, record bool) time.Duration {
	v, lt := storeChain(c.leaf, c.t, depth)
	places := 1
	if record {
		places++
	}
	var best time.Duration
	for try := range storeTries {
		r := storeRun()
		start := time.Now()
		for range places {
			if r.store(v, lt, site{}, nil) == nil {
				t.Fatalf("depth %d: store aborted", depth)
			}
		}
		if took := time.Since(start); try == 0 || took < best {
			best = took
		}
	}
	return best
}

// EVALUATION.md §4.3, §12.1; DECISIONS 513: storing a shared value takes linear time, as a let or in a record.
func TestStoreSharedScale(t *testing.T) {
	for _, record := range []bool{false, true} {
		for _, c := range storeLeafCases() {
			t.Run(fmt.Sprintf("%s/record=%v", c.name, record), func(t *testing.T) {
				small := storeTime(t, c, storeDepths[0], record)
				for _, d := range storeDepths[1:] {
					if took := storeTime(t, c, d, record); took > storeGrows*max(small, time.Millisecond) {
						t.Errorf("depth %d: %v, depth %d: %v, more than %dx", storeDepths[0], small, d, took, storeGrows)
					}
				}
			})
		}
	}
}
