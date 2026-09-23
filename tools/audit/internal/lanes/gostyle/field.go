package gostyle

import (
	"go/ast"

	"github.com/fantasim/canonlang/tools/audit/internal/finding"
	"github.com/fantasim/canonlang/tools/audit/internal/gosrc"
	"github.com/fantasim/canonlang/tools/audit/internal/lane"
	"github.com/fantasim/canonlang/tools/audit/internal/rules"
)

// fieldFindings is comment-field: a doc or trailing comment over FieldCommentLines on a
// struct field, or on a spec inside a parenthesized const/var/type group.
func fieldFindings(ctx *lane.Context, f *gosrc.File) []finding.Finding {
	if !ctx.On(ruleCommentField) {
		return nil
	}
	var out []finding.Finding
	for _, d := range f.AST.Decls {
		if gd, ok := d.(*ast.GenDecl); ok {
			out = append(out, groupedSpecFindings(ctx, f, gd)...)
			out = append(out, structFieldFindings(ctx, f, gd)...)
		}
	}
	return out
}

// groupedSpecFindings judges each spec's own Doc/Comment inside a parenthesized group;
// an ungrouped decl's doc belongs to its GenDecl instead (comment-decl).
func groupedSpecFindings(ctx *lane.Context, f *gosrc.File, gd *ast.GenDecl) []finding.Finding {
	if !gd.Lparen.IsValid() {
		return nil
	}
	var out []finding.Finding
	for _, sp := range gd.Specs {
		for _, g := range specComments(sp) {
			if n := contentLines(g); n > rules.FieldCommentLines {
				out = append(out, fieldFinding(ctx, f, g, specName(sp)))
			}
		}
	}
	return out
}

func specComments(sp ast.Spec) []*ast.CommentGroup {
	switch sp := sp.(type) {
	case *ast.TypeSpec:
		return nonNil(sp.Doc, sp.Comment)
	case *ast.ValueSpec:
		return nonNil(sp.Doc, sp.Comment)
	}
	return nil
}

type fieldGroup struct {
	field *ast.Field
	group *ast.CommentGroup
}

func structFieldFindings(ctx *lane.Context, f *gosrc.File, gd *ast.GenDecl) []finding.Finding {
	var out []finding.Finding
	for _, sp := range gd.Specs {
		st := structType(sp)
		if st == nil {
			continue
		}
		for _, fg := range structFields(st) {
			if n := contentLines(fg.group); n > rules.FieldCommentLines {
				out = append(out, fieldFinding(ctx, f, fg.group, fieldName(fg.field)))
			}
		}
	}
	return out
}

func structType(sp ast.Spec) *ast.StructType {
	ts, ok := sp.(*ast.TypeSpec)
	if !ok {
		return nil
	}
	st, ok := ts.Type.(*ast.StructType)
	if !ok || st.Fields == nil {
		return nil
	}
	return st
}

func structFields(st *ast.StructType) []fieldGroup {
	var out []fieldGroup
	for _, field := range st.Fields.List {
		for _, g := range nonNil(field.Doc, field.Comment) {
			out = append(out, fieldGroup{field, g})
		}
	}
	return out
}

func fieldFinding(ctx *lane.Context, f *gosrc.File, g *ast.CommentGroup, name string) finding.Finding {
	n := contentLines(g)
	line := ctx.Go.Line(g.Pos())
	fx := sizeFinding(ruleCommentField, f.Path, line, n, measured(n, unitLines, rules.FieldCommentLines))
	fx.Detail = name
	return fx
}
