package rules_test

import (
	"context"
	"fmt"
	"testing/fstest"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/views/rules"
)

// Check runs after check (DECISIONS 221): a view naming a field its record does not have is
// E1602 (VIEWMODEL.md G8).
func Example() {
	fsys := mapFS{
		"law/project.canon": &fstest.MapFile{Data: []byte("project demo {\n  canon: \"0.1\"\n}\n")},
		"law/a/a.canon": &fstest.MapFile{Data: []byte(
			"package a\n\nlocal record Item {\n  label: String\n}\n\nview Item {\n  colour \"Colour\"\n}\n")},
	}
	ctx := context.Background()
	p, err := build.Open(fsys, lawDir, build.Options{})
	if err != nil {
		fmt.Println(err)
		return
	}
	a, err := p.Analyze(ctx, nil)
	if err != nil {
		fmt.Println(err)
		return
	}
	bags := check.Bags{"a": a.Bag("a")}
	rules.Check(ctx, a.Program(), bags, "", nil)
	for _, f := range bags["a"].Findings() {
		fmt.Println(f.Code, f.Message)
	}
	// Output:
	// E1602 Item has no field, method, case or member named colour
}
