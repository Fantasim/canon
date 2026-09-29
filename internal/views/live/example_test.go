package live_test

import (
	"context"
	"fmt"
	"testing/fstest"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/eval"
	"github.com/fantasim/canonlang/internal/views/live"
)

// Evaluate heads each entry of a table by its view title, two equal titles shown with their key
// (API.md V6, VIEWMODEL.md S9); the headings are keyed by path relative to the table (V9).
func Example() {
	fsys := mapFS{
		"law/project.canon": &fstest.MapFile{Data: []byte("project demo {\n  canon: \"0.1\"\n}\n")},
		"law/a/a.canon": &fstest.MapFile{Data: []byte("package a\n\nrecord Pt {\n  name: String\n}\n\n" +
			"view Pt {\n  title \"Point {name}\"\n}\n\nlet pts: table Pt = {\n  p1 { name: \"A\" }\n  p2 { name: \"A\" }\n  p3 { name: \"B\" }\n}\n")},
	}
	p, err := build.Open(fsys, lawDir, build.Options{})
	if err != nil {
		fmt.Println(err)
		return
	}
	a, err := p.Analyze(context.Background(), nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	pts, _ := a.Force(eval.Root{Pkg: "a", Name: "pts"})
	in := live.Input{Program: a.Program(), Eval: standIn{info: a.Program().Info}}
	res, err := live.Evaluate(context.Background(), in, live.Target{Value: pts, Name: "pts"})
	if err != nil {
		fmt.Println(err)
		return
	}
	h := res.Headings
	fmt.Println(h["p1"].Title.Value, "|", h["p2"].Title.Value, "|", h["p3"].Title.Value)
	// Output: Point A (p1) | Point A (p2) | Point B
}
