package diagnostics

import (
	"fmt"
	"go/ast"
	"go/token"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/tools/audit/internal/finding"
	"github.com/fantasim/canonlang/tools/audit/internal/gosrc"
)

// literals reports every string literal, or constant concatenation of literals, that holds
// a code or a catalogued message text. Comments may cite codes: they are not literals.
func (s *scan) literals() {
	var segs []segment
	if s.cat != nil {
		segs = s.cat.segments(s.ctx.Limits.DiagTextWords)
	}
	for _, f := range s.files {
		ast.Inspect(f.AST, func(n ast.Node) bool {
			v, ok := foldString(n)
			if !ok {
				return true
			}
			s.judgeString(f, n, v, segs)
			return false
		})
	}
}

func (s *scan) judgeString(f *gosrc.File, n ast.Node, v string, segs []segment) {
	line := s.ctx.Go.Line(n.Pos())
	if m := reCodeToken.FindStringSubmatch(v); m != nil {
		s.emit(finding.Finding{
			Rule: ruleInline, File: f.Path, Line: line, Detail: detailCode,
			Message: fmt.Sprintf(msgCode, m[codeGroup]),
		})
		return
	}
	for _, sg := range segs {
		if strings.Contains(v, sg.text) {
			s.emit(finding.Finding{
				Rule: ruleInline, File: f.Path, Line: line, Detail: detailText,
				Message: fmt.Sprintf(msgText, sg.code, sg.text),
			})
			return
		}
	}
}

// foldString is the value of a string literal, or of a + chain of them, parentheses allowed.
func foldString(n ast.Node) (string, bool) {
	switch n := n.(type) {
	case *ast.BasicLit:
		if n.Kind != token.STRING {
			return "", false
		}
		v, err := strconv.Unquote(n.Value)
		return v, err == nil
	case *ast.ParenExpr:
		return foldString(n.X)
	case *ast.BinaryExpr:
		if n.Op != token.ADD {
			return "", false
		}
		x, okX := foldString(n.X)
		y, okY := foldString(n.Y)
		return x + y, okX && okY
	}
	return "", false
}
