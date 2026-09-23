package gostyle

import (
	"fmt"
	"go/ast"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/tools/audit/internal/finding"
	"github.com/fantasim/canonlang/tools/audit/internal/gosrc"
	"github.com/fantasim/canonlang/tools/audit/internal/lane"
)

// pkgDocFindings reports a package without a doc.go (pkg-doc), and a package other than main
// whose tests hold no Example function (pkg-example). A test-only directory is no package.
func pkgDocFindings(ctx *lane.Context) []finding.Finding {
	var out []finding.Finding
	for dir, files := range ctx.Go.Packages() {
		i := slices.IndexFunc(files, func(f *gosrc.File) bool { return !f.TestCode() })
		if i < 0 {
			continue
		}
		name := files[i].AST.Name.Name
		if ctx.On(rulePkgDoc) && !slices.ContainsFunc(files, isDocGo) {
			out = append(out, finding.Finding{Rule: rulePkgDoc, File: dir, Message: fmt.Sprintf(fmtNoDocGo, name)})
		}
		if ctx.On(rulePkgExample) && name != mainPkg && !slices.ContainsFunc(files, hasExample) {
			out = append(out, finding.Finding{Rule: rulePkgExample, File: dir, Message: fmt.Sprintf(fmtNoExample, name)})
		}
	}
	return out
}

// hasExample reports a _test.go file declaring a top-level Example function.
func hasExample(f *gosrc.File) bool {
	return f.Test && slices.ContainsFunc(f.AST.Decls, func(d ast.Decl) bool {
		fd, ok := d.(*ast.FuncDecl)
		return ok && fd.Recv == nil && strings.HasPrefix(fd.Name.Name, examplePrefix)
	})
}
