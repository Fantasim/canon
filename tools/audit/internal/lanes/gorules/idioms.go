package gorules

import (
	"fmt"
	"go/ast"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/tools/audit/internal/finding"
)

// logDirect reports log.Print*/Fatal*/Panic*, fmt.Print* and print/println outside cmd/
// and package main.
func (s *scan) logDirect() {
	for _, f := range s.files {
		if f.TestCode() || f.pkgName() == mainPkg || underCmd(f.Dir) {
			continue
		}
		ast.Inspect(f.AST, func(n ast.Node) bool {
			c, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if name := f.directLog(c); name != "" {
				s.emit(finding.Finding{
					Rule: ruleLogDirect, File: f.Path, Line: s.line(c.Pos()), Detail: name,
					Message: fmt.Sprintf(msgLogDirect, name), Fix: fixLogDirect,
				})
			}
			return true
		})
	}
}

// directLog names the unstructured print call c makes, or "".
func (f *src) directLog(c *ast.CallExpr) string {
	if id, ok := c.Fun.(*ast.Ident); ok && id.Obj == nil && slices.Contains(builtinPrint, id.Name) {
		return id.Name
	}
	pkg, name := f.callee(c)
	switch {
	case pkg == pkgLog && slices.ContainsFunc(logPrefixes, func(p string) bool { return strings.HasPrefix(name, p) }),
		pkg == pkgFmt && slices.Contains(fmtPrints, name):
		return pkg + dirHere + name
	}
	return ""
}

// bareGoroutine reports every go statement outside the safego package itself.
func (s *scan) bareGoroutine() {
	for _, f := range s.files {
		if f.TestCode() || strings.HasSuffix(s.importPath(f.Dir), safegoSuffix) {
			continue
		}
		ast.Inspect(f.AST, func(n ast.Node) bool {
			g, ok := n.(*ast.GoStmt)
			if !ok {
				return true
			}
			what := funcLitName
			if _, lit := g.Call.Fun.(*ast.FuncLit); !lit {
				what = s.source(f, g.Call.Fun)
			}
			s.emit(finding.Finding{
				Rule: ruleBareGo, File: f.Path, Line: s.line(g.Pos()), Detail: what,
				Message: fmt.Sprintf(msgBareGo, what), Fix: fixBareGo,
			})
			return true
		})
	}
}
