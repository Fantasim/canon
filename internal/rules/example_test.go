package rules_test

import (
	"context"
	"fmt"

	"github.com/fantasim/canonlang/internal/check"
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

// A build keeps one IndexCache across the programs of its snapshots: each index walks only the
// files the cache has not seen, and relates each check finding to the declaration in its file.
func ExampleIndexCache() {
	fs := &source.FileSet{}
	src, _ := fs.Add("deck.canon", "/deck.canon", []byte("package teamboard\n\nrecord Deck {\n  check short: false else \"too long\"\n}\n"))
	bag := diag.NewBag(fs, "teamboard")
	file := syntax.Parse(src, syntax.FileSource, bag)
	short := file.Decls[0].(*syntax.RecordDecl).Body.Items[0].(*syntax.CheckDecl)
	deck := &types.RecordType{Pkg: "teamboard", Name: "Deck", Checks: []*syntax.CheckDecl{short}}
	prog := &check.Program{Packages: []*check.Package{{Path: "teamboard", Files: []*syntax.File{file}}}}
	cache := &rules.IndexCache{}
	cache.Index(prog)
	ev := &evaluator{scripts: map[*syntax.CheckDecl]script{short: fails("too long")}}
	r := rules.NewShared(cache.Index(prog), ev, map[string]*diag.Bag{"teamboard": bag})
	r.Instances(context.Background(), eval.Root{Pkg: "teamboard", Name: "deck"}, &value.Record{T: deck})
	f := bag.Findings()[0]
	fmt.Println(f.Code, f.Path, f.Check, f.Related[0].Note)
	// Output: E5001 deck short check short
}
