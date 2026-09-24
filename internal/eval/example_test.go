package eval_test

import (
	"context"
	"fmt"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// served is a fixture host, as tests of eval use before load and verify exist: it serves no
// file and finds every value valid.
type served struct{}

func (served) Load(context.Context, *syntax.LoadExpr, types.Type) (value.Value, bool) {
	return nil, false
}

func (served) Verify(context.Context, eval.Root, value.Value) bool { return true }

// A build checks with eval's folder, then forces each top-level value; the refinement bound
// FARM_MAX_MODELS is folded by the same evaluator.
func Example() {
	ctx := context.Background()
	fs := &source.FileSet{}
	src, _ := fs.Add("farm/farm.canon", "/farm/farm.canon", []byte(`/// Farms.
package farm

const FARM_MAX_MODELS = 100

/// How many models a farm may hold.
let maxModels: Int(0..=FARM_MAX_MODELS) = FARM_MAX_MODELS - 1

/// The first squares.
let squares: [Int] = [n * n for n in 1..=4]
`))
	files := []*syntax.File{syntax.Parse(src, syntax.FileSource, diag.NewBag(fs, ""))}
	bags := check.Bags{}
	prog := check.Check(ctx, project.New("demo", project.Version{Minor: 1}), files, bags, eval.NewFolder(bags, eval.Options{}))
	ev := eval.New(prog, served{}, bags, eval.Options{})
	for _, name := range []string{"FARM_MAX_MODELS", "maxModels", "squares"} {
		v, _ := ev.Force(ctx, eval.Root{Pkg: "farm", Name: name})
		fmt.Print(name, " = ", v.CanonText(), "; ")
	}
	ev.BeginVerification(ctx)
	fmt.Println(len(bags["farm"].Findings()), "findings")
	// Output: FARM_MAX_MODELS = 100; maxModels = 99; squares = [1, 4, 9, 16]; 0 findings
}
