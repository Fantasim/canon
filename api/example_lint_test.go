package canon_test

import (
	"context"
	"fmt"

	canon "github.com/fantasim/canonlang/api"
)

// lintLaw is a project whose one record keeps a legacy prefix in a field name (API.md L2).
var lintLaw = map[string][]byte{
	"/law/project.canon":     []byte("project acme {\n  canon: \"0.1\"\n}\n"),
	"/law/units/units.canon": []byte("package units\n\nrecord Unit {\n  dwHealth: Int\n}\n\nlet units: [Unit] = [{ dwHealth: 3 }]\n"),
}

// API.md L1, L11 (DECISIONS 440): Lint reports the lint findings of the selected packages and,
// after them, the fixes, each a complete request based on the revision linted; applied with Edit,
// it is current, and linting again reports nothing.
func ExampleProject_Lint() {
	ctx := context.Background()
	p, err := canon.Open("/law", canon.Options{FS: newMemFS(lintLaw), Cache: "off"})
	if err != nil {
		fmt.Println(err)
		return
	}
	defer p.Close()
	res, err := p.Lint(ctx, "units")
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, f := range res.Findings {
		fmt.Println(f.Severity, f.Code, f.File, f.Line, f.Col, res.Summary.Warnings, len(res.Fixes))
	}
	fix := res.Fixes[0]
	fmt.Println(fix.Code, fix.File, fix.Line, fix.Col, fix.Edit.Base == res.Revision, fix.Edit.Ops[0].Kind, fix.Edit.Ops[0].Path, fix.Edit.Ops[0].Name)
	applied, err := p.Edit(ctx, fix.Edit)
	if err != nil {
		fmt.Println(err)
		return
	}
	again, err := p.Lint(ctx, "units")
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Println(applied.Applied, len(again.Findings), len(again.Fixes), again.Revision == applied.Revision)
	// Output: warning W0001 units/units.canon 4 3 1 1
	// W0001 units/units.canon 4 3 true renameName units:Unit.dwHealth health
	// true 0 0 true
}
