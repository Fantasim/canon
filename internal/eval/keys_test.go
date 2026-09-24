package eval_test

import (
	"fmt"
	"testing"

	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/value"
)

// keysSource maps a string-literal union over a ref: `sword:` is the ref, `"all":` the literal.
const keysSource = `/// A.
package a

/// An item.
record Item {
  /// Its power.
  power: Int = 1
}

/// The items.
let items: table Item = { sword {} }

/// A weight target.
type Target = ref Item | "all"

/// The weights.
let weights: {Target: Int} = { sword: 1, "all": 2 }
`

// TYPES.md §5.2, §13.2: `sword:` keyed by a literal union over a ref builds the ref, never an internal error.
func TestLitUnionRefKey(t *testing.T) {
	b := runBuild(t, parseFiles(t, []string{"a/a.canon"}, [][]byte{[]byte(keysSource)}), eval.Options{})
	v, ok := b.values[eval.Root{Pkg: "a", Name: "weights"}]
	if !ok {
		t.Fatalf("a.weights did not evaluate:\n%s", b.findings(t))
	}
	m, ok := v.(*value.Map)
	if !ok || len(m.Keys) == 0 {
		t.Fatalf("a.weights = %s, want its keys", v.CanonText())
	}
	if got := fmt.Sprintf("%T %s", m.Keys[0], m.Keys[0].CanonText()); got != wantRefKey {
		t.Errorf("first key = %s, want %s", got, wantRefKey)
	}
}

// wantRefKey is the first key of a.weights, `sword:` resolved against the union's ref.
const wantRefKey = "*value.Ref sword"
