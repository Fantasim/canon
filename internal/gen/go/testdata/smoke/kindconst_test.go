package kinds

import (
	"fmt"
	"testing"

	reward "example.com/data/demo/reward/out/go"
)

// CODEGEN.md §4.4, §5.1, DECISIONS 438: the constants of a variant's kind are the kind enum's members, a list of kinds a function.
func TestKindConstants(t *testing.T) {
	got := fmt.Sprintf("%v %v %d %d %s", K == ShapeKindDot, Bk == reward.RewardKindGold, Ks().Len(), Eight().Len(), K.String())
	if want := "true true 2 8 dot"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if Ks().At(1) != ShapeKindBox || Eight().At(7) != ShapeKindBox {
		t.Errorf("a list holds %v and %v", Ks().At(1), Eight().At(7))
	}
}
