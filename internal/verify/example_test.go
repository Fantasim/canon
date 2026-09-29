package verify_test

import (
	"context"
	"fmt"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/types"
	"github.com/fantasim/canonlang/internal/value"
	"github.com/fantasim/canonlang/internal/verify"
)

// Stage B walks `let deck: Deck = { maxHidden: 0 }` against `maxHidden: Int(1..)`, reports the
// value outside its range and marks it invalid, so its record's checks will be skipped.
func Example() {
	hidden := &types.Refined{Of: types.IntType, Range: &types.Bound{Lo: types.Limit{I: 1}, HasLo: true}}
	deck := &types.RecordType{Pkg: "teamboard", Name: "Deck", Fields: []*types.Field{{Name: "maxHidden", Type: hidden}}}
	zero := &value.Int{V: 0, T: hidden}
	ev := &evaluator{}
	bag := diag.NewBag(&source.FileSet{}, "teamboard")
	v := verify.New(ev, nil, map[string]*diag.Bag{"teamboard": bag}, nil)
	res, err := v.Check(context.Background(), eval.Root{Pkg: "teamboard", Name: "deck"}, &value.Record{T: deck, Fields: []value.Value{zero}})
	f := bag.Findings()[0]
	fmt.Println(res.Valid, err, f.Code, f.Path, ev.invalid[0] == zero)
	// Output: false <nil> E3204 deck.maxHidden true
}

// A build keeps one IndexCache across the programs of its snapshots: each index scans only the
// files the cache has not seen, and relates a finding to the type written for the value.
func ExampleIndexCache() {
	fs := &source.FileSet{}
	src, _ := fs.Add("deck.canon", "/deck.canon", []byte("package teamboard\n\nrecord Deck {\n  maxHidden: Int(1..)\n}\n"))
	bag := diag.NewBag(fs, "teamboard")
	file := syntax.Parse(src, syntax.FileSource, bag)
	written := file.Decls[0].(*syntax.RecordDecl).Body.Items[0].(*syntax.FieldDecl).Type
	hidden := &types.Refined{Of: types.IntType, Range: &types.Bound{Lo: types.Limit{I: 1}, HasLo: true}}
	deck := &types.RecordType{Pkg: "teamboard", Name: "Deck", Fields: []*types.Field{{Name: "maxHidden", Type: hidden}}}
	info := &check.Info{TypeExprs: map[syntax.Type]types.Type{written: hidden}}
	prog := &check.Program{Packages: []*check.Package{{Path: "teamboard", Files: []*syntax.File{file}}}, Info: info}
	cache := &verify.IndexCache{}
	cache.Index(prog)
	v := verify.NewShared(cache.Index(prog), &evaluator{}, map[string]*diag.Bag{"teamboard": bag}, nil)
	zero := &value.Int{V: 0, T: hidden}
	_, err := v.Check(context.Background(), eval.Root{Pkg: "teamboard", Name: "deck"}, &value.Record{T: deck, Fields: []value.Value{zero}})
	f := bag.Findings()[0]
	line, col := fs.Position(f.Related[0].Span.File, f.Related[0].Span.Start)
	fmt.Println(err, f.Code, f.Path, line, col)
	// Output: <nil> E3204 deck.maxHidden 4 3
}
