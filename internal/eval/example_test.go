package eval_test

import (
	"context"
	"fmt"

	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// served is a fixture host, as tests of eval use before load and verify exist: it serves each
// load expression a value from memory and finds every value valid.
type served map[*syntax.LoadExpr]value.Value

func (s served) Load(_ context.Context, e *syntax.LoadExpr, _ types.Type) (value.Value, bool) {
	v, ok := s[e]
	return v, ok
}

func (served) Verify(context.Context, eval.Root, value.Value) bool { return true }

// The evaluator reaches load and verify only through its host: `let maxModels: Int =
// load(…)` is decoded by the host, then verified as the top-level value it initializes.
func Example() {
	ctx := context.Background()
	call := &syntax.LoadExpr{}
	var host eval.Host = served{call: &value.Int{V: 12, T: types.IntType}}
	v, ok := host.Load(ctx, call, types.IntType)
	_, missing := host.Load(ctx, &syntax.LoadExpr{}, types.IntType)
	root := eval.Root{Pkg: "resource.farm", Name: "maxModels"}
	fmt.Println(v.CanonText(), ok, missing, root.Pkg+"."+root.Name, host.Verify(ctx, root, v))
	// Output: 12 true false resource.farm.maxModels true
}
