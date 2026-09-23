package gostyle

import (
	"go/ast"
	"go/token"

	"github.com/fantasim/canonlang/tools/audit/internal/finding"
	"github.com/fantasim/canonlang/tools/audit/internal/gosrc"
	"github.com/fantasim/canonlang/tools/audit/internal/lane"
)

// declFindings is comment-decl: a doc comment over DocLines on a top-level FuncDecl, or
// on a GenDecl (a const/var/type group's own doc, not a spec inside it: that is comment-field).
func declFindings(ctx *lane.Context, f *gosrc.File) []finding.Finding {
	if !ctx.On(ruleCommentDecl) {
		return nil
	}
	var out []finding.Finding
	for _, d := range f.AST.Decls {
		doc := declDoc(d)
		if doc == nil {
			continue
		}
		if n := contentLines(doc); n > ctx.Limits.CommentDeclLines {
			line := ctx.Go.Line(doc.Pos())
			out = append(out, sizeFinding(ruleCommentDecl, f.Path, line, n, measured(n, unitLines, ctx.Limits.CommentDeclLines)))
		}
	}
	return out
}

// declDoc is the doc comment a top-level declaration owns, nil for the import block.
func declDoc(d ast.Decl) *ast.CommentGroup {
	switch d := d.(type) {
	case *ast.FuncDecl:
		return d.Doc
	case *ast.GenDecl:
		if d.Tok == token.IMPORT {
			return nil
		}
		return d.Doc
	}
	return nil
}
