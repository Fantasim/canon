package lint_test

import (
	"context"
	"fmt"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/lint"
)

// Run reads the analysis a check makes and reports the lint warnings of the selected packages,
// and the edit request that fixes each one a request can fix.
func ExampleRun() {
	fsys := mapFS{
		"law/project.canon": file("project acme {\n  canon: \"0.1\"\n}\n"),
		"law/a/a.canon":     file("package a\n\nrecord R {\n  dwCount: Int\n}\n"),
	}
	p, err := build.Open(fsys, "/law", build.Options{})
	if err != nil {
		fmt.Println(err)
		return
	}
	a, err := p.Analyze(context.Background(), []string{"a"})
	if err != nil {
		fmt.Println(err)
		return
	}
	res, err := lint.Run(context.Background(), a, edit.NewSnapshot(a))
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, f := range res.Findings {
		fmt.Println(f.Code, f.Package, f.Message)
	}
	for _, f := range res.Fixes {
		fmt.Println(f.Code, f.Ops[0].Path, f.Ops[0].Name)
	}
	// Output:
	// W0001 a field dwCount keeps the legacy prefix dw in its Canon name: rename it count (canon rename keeps the old wire name with @json where a load or a data emit reads the field)
	// W0001 a:R.dwCount count
}
