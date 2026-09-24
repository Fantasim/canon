package ir

import (
	"context"
	"math/big"
	"testing"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// countingFolder folds every expression to 1 and counts its calls.
type countingFolder struct{ calls int }

func (f *countingFolder) Fold(context.Context, check.Object, syntax.Expr, *check.Info) (value.Value, bool) {
	f.calls++
	return &value.Int{V: 1}, true
}

// owner stands for a record's check object; only its identity matters here.
type owner struct{ check.Object }

// TestFieldDefaultSkipsBrokenOwner is DECISIONS 209 and 213: a broken record's field defaults are never folded (log-2026-09-24 "A3 eval": the E3022 archives overflowed through such a fold); a sound owner's constant default is.
func TestFieldDefaultSkipsBrokenOwner(t *testing.T) {
	for _, broken := range []bool{true, false} {
		o := &owner{}
		fold := &countingFolder{}
		s := &stage{ctx: context.Background(), in: Input{Fold: fold}, info: &check.Info{Broken: map[check.Object]bool{o: broken}}}
		fd := &Field{}
		s.fieldDefault(fd, &types.Field{Default: &syntax.IntLit{Value: big.NewInt(1)}}, o, nil)
		if folded := fold.calls > 0 || fd.Default != nil; folded == broken {
			t.Errorf("broken owner %v: folded %v (%d calls)", broken, folded, fold.calls)
		}
	}
}
