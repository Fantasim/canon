package value_test

import (
	"math"
	"testing"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// hashGap are the equal rows hashed apart: the TYPES.md §7.5 non-transitivity gap (a Louis-call).
var hashGap = map[string]bool{"entries by identity": true, "ref and entry": true}

// negZero is -0.0.
func negZero() *value.Float { return &value.Float{V: math.Copysign(0, -1), T: types.FloatType} }

func zero() *value.Float { return &value.Float{V: 0, T: types.FloatType} }

// TYPES.md §7.5, DECISIONS 199: equal values hash alike, reordered and nested maps included.
func TestHashOfEqualValues(t *testing.T) {
	cases := []struct {
		name string
		a, b value.Value
	}{
		{"reordered maps", pairs(str("a"), num(1), str("b"), num(2)), pairs(str("b"), num(2), str("a"), num(1))},
		{"nested reordered maps", pairs(str("m"), pairs(str("a"), num(1), str("b"), num(2))), pairs(str("m"), pairs(str("b"), num(2), str("a"), num(1)))},
		{"reordered maps in a list", list(pairs(str("a"), num(1), str("b"), num(2))), list(pairs(str("b"), num(2), str("a"), num(1)))},
		{"-0.0 as a key", pairs(negZero(), num(1)), pairs(zero(), num(1))},
		{"-0.0 as a value", pairs(str("a"), negZero()), pairs(str("a"), zero())},
	}
	for _, c := range equalCases {
		if c.want && !hashGap[c.name] {
			cases = append(cases, struct {
				name string
				a, b value.Value
			}{c.name, c.a, c.b})
		}
	}
	for _, c := range cases {
		if !value.Equal(c.a, c.b) {
			t.Errorf("%s: not equal", c.name)
		}
		if value.Hash(c.a) != value.Hash(c.b) {
			t.Errorf("%s: equal values hash apart", c.name)
		}
	}
}

// hashWide is the length of the wide list the bounded hash is measured over.
const hashWide = 100_000

// hashAllocs is the most allocations one Hash may make, whatever the value's size.
const hashAllocs = 8

// DECISIONS 199: Hash walks a bounded prefix, so its allocations do not grow with a list of
// 10^5 elements, alone or inside [[i], big].
func TestHashIsBounded(t *testing.T) {
	elems := make([]value.Value, hashWide)
	for i := range elems {
		elems[i] = num(int64(i))
	}
	big := list(elems...)
	outer := &value.List{T: &types.ListType{Elem: types.AnyType}, Elems: []value.Value{list(num(1)), big}}
	for name, v := range map[string]value.Value{"big": big, "[[i], big]": outer} {
		if allocs := testing.AllocsPerRun(10, func() { value.Hash(v) }); allocs > hashAllocs {
			t.Errorf("Hash(%s) allocates %v times", name, allocs)
		}
	}
}
