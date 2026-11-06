package rules

import (
	"context"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// invalidSet is an Evaluator telling the values it holds invalid, running no check.
type invalidSet map[value.Value]bool

func (s invalidSet) Invalid(v value.Value) bool { return s[v] }

func (invalidSet) Run(context.Context, *syntax.CheckDecl, value.Value, string) Run { return Run{} }

// DECISIONS 507: the signature tells sharing apart, so equal content shared otherwise never signs alike.
func TestSignatureTellsSharing(t *testing.T) {
	bad := invalidSet{}
	num := func(invalid bool) value.Value {
		n := &value.Int{V: 1, T: types.IntType}
		bad[n] = invalid
		return n
	}
	list := func(vs ...value.Value) value.Value { return &value.List{Elems: vs} }
	shared := func() value.Value { // [L, L], L = [x], x invalid
		l := list(num(true))
		return list(l, l)
	}
	apart := list(list(num(true)), list(num(false))) // [[x], [y]], only x invalid
	first, again := signOf(bad, shared()), signOf(bad, shared())
	if !slices.Equal(first, again) {
		t.Errorf("equal sharing: %v and %v, want equal", first, again)
	}
	if other := signOf(bad, apart); slices.Equal(first, other) {
		t.Errorf("shared %v and apart %v sign alike", first, other)
	}
	a := list(num(true))                                                        // A = [x], x invalid
	twice, around := list(a, a, list(num(false))), list(a, list(num(false)), a) // [A, A, C], [A, B, A]
	if x, y := signOf(bad, twice), signOf(bad, around); slices.Equal(x, y) {
		t.Errorf("[A, A, C] %v and [A, B, A] %v sign alike", x, y)
	}
}
