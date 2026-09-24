package value_test

import (
	"math"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// deepDepth is how deep the nested list goes: past what a recursive walk stands in deepStack.
const deepDepth = 100_000

// deepStack is the host stack the deep walks run under: a recursive walk 10^5 deep overflows
// it and dies, an iterative one does not need it. No test of this package runs in parallel.
const deepStack = 8 << 20

// towerLevels is how many times a tower doubles its sharing: 2^60 paths, 61 distinct nodes.
const towerLevels = 60

// deepList is [[…[leaf]…]], depth lists around leaf.
func deepList(depth int, leaf int64) value.Value {
	var v value.Value = num(leaf)
	for range depth {
		v = &value.List{T: &types.ListType{Elem: types.AnyType}, Elems: []value.Value{v}}
	}
	return v
}

// tower is levels lists each holding the one below twice, over [leaf].
func tower(levels int, leaf int64) value.Value {
	var v value.Value = list(num(leaf))
	for range levels {
		v = &value.List{T: &types.ListType{Elem: types.AnyType}, Elems: []value.Value{v, v}}
	}
	return v
}

// DECISIONS 197: equality, the text form, its length and the hash walk a value 10^5 lists
// deep under a host stack a recursive walk would overflow.
func TestDeepValues(t *testing.T) {
	a, b, c := deepList(deepDepth, 1), deepList(deepDepth, 1), deepList(deepDepth, 2)
	want := strings.Repeat("[", deepDepth) + "1" + strings.Repeat("]", deepDepth)
	defer debug.SetMaxStack(debug.SetMaxStack(deepStack))
	if !value.Equal(a, b) || value.Equal(a, c) {
		t.Error("equality over a deep list")
	}
	if eq, ok, _ := value.EqualUpTo(a, b, math.MaxInt); !eq || !ok {
		t.Error("EqualUpTo over a deep list")
	}
	if a.CanonText() != want || value.TextLenUpTo(a, math.MaxInt) != len(want) {
		t.Error("text of a deep list")
	}
	if value.Hash(a) != value.Hash(b) {
		t.Error("equal deep lists hash apart")
	}
}

// DECISIONS 197: two towers built apart, each shared 2^60 ways, compare once per pair of
// distinct nodes; the same instance compares at once.
func TestSharedTowers(t *testing.T) {
	l, m := tower(towerLevels, 1), tower(towerLevels, 1)
	if !value.Equal(l, m) || !value.Equal(l, l) {
		t.Error("equal towers compare unequal")
	}
	if value.Equal(l, tower(towerLevels, 2)) {
		t.Error("towers that differ at the bottom compare equal")
	}
	small := tower(3, 1).CanonText()
	if small != "[[[[1], [1]], [[1], [1]]], [[[1], [1]], [[1], [1]]]]" {
		t.Errorf("text of a small tower: %s", small)
	}
}

// DECISIONS 197: the text length is counted without the text, and stops at its limit even
// over a tower whose text would be 2^60 times longer.
func TestTextLenUpTo(t *testing.T) {
	small := tower(3, 1)
	if got := value.TextLenUpTo(small, 1000); got != len(small.CanonText()) {
		t.Errorf("TextLenUpTo = %d, want %d", got, len(small.CanonText()))
	}
	if got := value.TextLenUpTo(str("a\"b"), 1000); got != len("a\"b") {
		t.Errorf("a top-level string counts raw: %d", got)
	}
	if got := value.TextLenUpTo(tower(towerLevels, 1), 1000); got != 1000 {
		t.Errorf("TextLenUpTo over a tower = %d, want the limit", got)
	}
}

// DECISIONS 197: a text cut at its limit stops the walk there, at a character boundary.
func TestTextUpTo(t *testing.T) {
	if got := value.TextUpTo(tower(towerLevels, 1), 10); got != "[[[[[[[[[[" {
		t.Errorf("TextUpTo over a tower = %q", got)
	}
	if got := value.TextUpTo(list(str("é")), 3); got != `["` {
		t.Errorf("TextUpTo splits a character: %q", got)
	}
	if got := value.TextUpTo(num(42), 10); got != "42" {
		t.Errorf("TextUpTo of a scalar = %q", got)
	}
}

// DECISIONS 199: EqualUpTo counts every pair it compares, scalar pairs included, and stops at
// its limit; an entry against an entry is settled by identity, allocating nothing.
func TestEqualUpTo(t *testing.T) {
	nested := func(n int64) value.Value { return list(list(num(n)), list(num(n))) }
	if eq, ok, n := value.EqualUpTo(nested(1), nested(1), 10); !eq || !ok || n != 5 {
		t.Errorf("EqualUpTo = %t %t %d, want true true 5", eq, ok, n)
	}
	if eq, ok, n := value.EqualUpTo(nested(1), nested(1), 2); eq || ok || n != 2 {
		t.Errorf("EqualUpTo past its limit = %t %t %d, want false false 2", eq, ok, n)
	}
	if eq, ok, n := value.EqualUpTo(num(1), num(1), 1); !eq || !ok || n != 1 {
		t.Errorf("a scalar pair is one pair: %t %t %d", eq, ok, n)
	}
	a, b := entry("open", "Open"), entry("open", "Other")
	if allocs := testing.AllocsPerRun(100, func() { value.Equal(a, b) }); allocs != 0 {
		t.Errorf("an entry against an entry allocates %v times", allocs)
	}
}

// DECISIONS 199: a map finds a key through its hash index, the -0.0 key included, and two
// maps of many keys in opposite orders compare in time linear in their size.
func TestMapIndex(t *testing.T) {
	const n = 100_000
	a, b := pairs(), pairs()
	for i := range int64(n) {
		a.Keys, a.Vals = append(a.Keys, num(i)), append(a.Vals, num(i))
		b.Keys, b.Vals = append(b.Keys, num(n-1-i)), append(b.Vals, num(n-1-i))
	}
	if v, ok := a.Get(num(n - 1)); !ok || !value.Equal(v, num(n-1)) {
		t.Errorf("Get(n-1) = %v, %t", v, ok)
	}
	if eq, ok, compared := value.EqualUpTo(a, b, 10*n); !eq || !ok || compared != 1+2*n {
		t.Errorf("EqualUpTo of reordered maps = %t %t %d", eq, ok, compared)
	}
	f := &value.Map{T: &types.MapType{Key: types.FloatType, Value: types.IntType}, Keys: []value.Value{&value.Float{V: 0, T: types.FloatType}}, Vals: []value.Value{num(1)}}
	if _, ok := f.Get(&value.Float{V: math.Copysign(0, -1), T: types.FloatType}); !ok {
		t.Error("-0.0 misses the key 0.0")
	}
}
