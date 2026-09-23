package gostyle

import (
	"go/ast"

	"github.com/fantasim/canonlang/tools/audit/internal/finding"
	"github.com/fantasim/canonlang/tools/audit/internal/gosrc"
	"github.com/fantasim/canonlang/tools/audit/internal/lane"
)

// blockFindings is comment-block: a comment group inside a function body spanning over
// CommentBlock lines. Every over-limit block in one function shares its ratchet key (rule,
// file, symbol), so extra blocks add to the count instead of opening a new baseline row.
func blockFindings(ctx *lane.Context, f *gosrc.File) []finding.Finding {
	if !ctx.On(ruleCommentBlock) {
		return nil
	}
	bodies := funcBodies(f.AST)
	var out []finding.Finding
	for _, g := range f.AST.Comments {
		if !insideAny(bodies, g) {
			continue
		}
		if n := contentLines(g); n > ctx.Limits.CommentBlockLines {
			line := ctx.Go.Line(g.Pos())
			out = append(out, sizeFinding(ruleCommentBlock, f.Path, line, n, measured(n, unitLines, ctx.Limits.CommentBlockLines)))
		}
	}
	return out
}

// funcBodies lists every top-level function or method body; a comment inside a closure
// nested in one of these still falls inside its span.
func funcBodies(file *ast.File) []*ast.BlockStmt {
	var out []*ast.BlockStmt
	for _, d := range file.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Body != nil {
			out = append(out, fd.Body)
		}
	}
	return out
}

func insideAny(bodies []*ast.BlockStmt, g *ast.CommentGroup) bool {
	for _, b := range bodies {
		if g.Pos() >= b.Lbrace && g.End() <= b.Rbrace {
			return true
		}
	}
	return false
}
