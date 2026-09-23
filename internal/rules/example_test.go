package rules_test

import (
	"context"
	"fmt"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/rules"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
)

// Stage C runs `check not layouts.isEmpty() else "at least one layout"` on each Deck instance
// reachable from the top-level values; the evaluator says how each run went.
func Example() {
	notEmpty := &syntax.CheckDecl{Keyword: syntax.KwCheck}
	deck := &types.RecordType{Pkg: "teamboard", Name: "Deck", Checks: []*syntax.CheckDecl{notEmpty}}
	ev := &evaluator{scripts: map[*syntax.CheckDecl]script{notEmpty: fails("at least one layout")}}
	bag := diag.NewBag(&source.FileSet{}, "teamboard")
	r := rules.New(ev, nil, map[string]*diag.Bag{"teamboard": bag})
	r.Instances(context.Background(), eval.Root{Pkg: "teamboard", Name: "deck"}, &value.Record{T: deck})
	f := bag.Findings()[0]
	fmt.Println(f.Code, f.Path, f.Message)
	// Output: E5001 deck at least one layout
}
