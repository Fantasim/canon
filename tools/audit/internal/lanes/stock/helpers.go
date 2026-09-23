package stock

import (
	"go/ast"
	"strings"

	"github.com/fantasim/canonlang/tools/audit/internal/gosrc"
)

// ownedByGorules says a modernize slicescontains issue sits in a function that IS the
// contains/index helper: gorules reports that function whole ("delete it, use
// slices.Contains"), so the loop-level twin would count the same thing twice.
func ownedByGorules(tree *gosrc.Tree, file string, iss golangciIssue) bool {
	if tree == nil || iss.FromLinter != linterModernize || !strings.HasPrefix(iss.Text, modernizeContains) {
		return false
	}
	f, ok := tree.File(file)
	if !ok {
		return false
	}
	for _, d := range f.AST.Decls {
		fd, ok := d.(*ast.FuncDecl)
		if ok && fd.Body != nil && tree.Line(fd.Pos()) <= iss.Pos.Line && iss.Pos.Line <= tree.Line(fd.End()) {
			return isWholeBodyLoop(fd.Body.List)
		}
	}
	return false
}

func isWholeBodyLoop(body []ast.Stmt) bool {
	if len(body) != helperBodyLen {
		return false
	}
	_, loop := body[0].(*ast.RangeStmt)
	_, ret := body[1].(*ast.ReturnStmt)
	return loop && ret
}
