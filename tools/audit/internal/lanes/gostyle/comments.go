package gostyle

import (
	"fmt"
	"go/ast"
	"go/token"
	"path"

	"github.com/fantasim/canonlang/tools/audit/internal/finding"
	"github.com/fantasim/canonlang/tools/audit/internal/gosrc"
	"github.com/fantasim/canonlang/tools/audit/internal/lane"
	"github.com/fantasim/canonlang/tools/audit/internal/rules"
)

// commentFindings runs every comment rule over the parsed tree, file by file.
func commentFindings(ctx *lane.Context) []finding.Finding {
	var out []finding.Finding
	for _, f := range ctx.Go.Files {
		if !f.Generated {
			out = append(out, fileCommentFindings(ctx, f)...)
		}
	}
	return out
}

// fileCommentFindings runs the placement rules first; a comment one of them flagged is
// not judged again by comment-adr-narration or counted by comment-ratio.
func fileCommentFindings(ctx *lane.Context, f *gosrc.File) []finding.Finding {
	var out []finding.Finding
	out = append(out, declFindings(ctx, f)...)
	out = append(out, fieldFindings(ctx, f)...)
	out = append(out, headerFinding(ctx, f)...)
	out = append(out, pkgDocFinding(ctx, f)...)
	out = append(out, blockFindings(ctx, f)...)
	flagged := flaggedGroups(ctx, f, out)
	out = append(out, ratioFinding(ctx, f, flagged)...)
	return append(out, contentFindings(ctx, f, flagged)...)
}

// flaggedGroups is the set of comment groups a placement finding already reported.
func flaggedGroups(ctx *lane.Context, f *gosrc.File, placement []finding.Finding) map[*ast.CommentGroup]bool {
	lines := map[int]bool{}
	for _, fx := range placement {
		lines[fx.Line] = true
		if fx.Rule == ruleCommentHeader {
			groups, _ := headerGroups(f.AST)
			for _, g := range groups {
				lines[ctx.Go.Line(g.Pos())] = true
			}
		}
	}
	out := map[*ast.CommentGroup]bool{}
	for _, g := range f.AST.Comments {
		if lines[ctx.Go.Line(g.Pos())] {
			out[g] = true
		}
	}
	return out
}

func isDocGo(f *gosrc.File) bool { return path.Base(f.Path) == docGoFile }

// headerFinding is comment-file-header: every comment above the package clause, or
// between it and the first real declaration, outside doc.go. One finding per file.
func headerFinding(ctx *lane.Context, f *gosrc.File) []finding.Finding {
	if !ctx.On(ruleCommentHeader) || isDocGo(f) {
		return nil
	}
	groups, total := headerGroups(f.AST)
	if total == 0 {
		return nil
	}
	line := ctx.Go.Line(groups[0].Pos())
	return []finding.Finding{sizeFinding(ruleCommentHeader, f.Path, line, total, fmt.Sprintf(fmtHeaderLines, total))}
}

func headerGroups(file *ast.File) ([]*ast.CommentGroup, int) {
	boundary := headerBoundary(file)
	var groups []*ast.CommentGroup
	total := 0
	for _, g := range file.Comments {
		if g == file.Doc && contentLines(g) <= rules.DocLines {
			continue
		}
		if g.End() < boundary {
			groups = append(groups, g)
			total += contentLines(g)
		}
	}
	return groups, total
}

// headerBoundary is where the first real (non-import) declaration's own territory
// begins: its doc comment's start when it has one, else the declaration itself.
func headerBoundary(file *ast.File) token.Pos {
	for _, d := range file.Decls {
		if g, ok := d.(*ast.GenDecl); ok && g.Tok == token.IMPORT {
			continue
		}
		if doc := declDoc(d); doc != nil {
			return doc.Pos()
		}
		return d.Pos()
	}
	return file.End()
}

// pkgDocFinding is comment-pkg: doc.go's package doc comment over PkgDocLines.
func pkgDocFinding(ctx *lane.Context, f *gosrc.File) []finding.Finding {
	if !ctx.On(ruleCommentPkg) || !isDocGo(f) || f.AST.Doc == nil {
		return nil
	}
	n := contentLines(f.AST.Doc)
	if n <= rules.PkgDocLines {
		return nil
	}
	line := ctx.Go.Line(f.AST.Doc.Pos())
	msg := measured(n, unitLines, rules.PkgDocLines)
	return []finding.Finding{sizeFinding(ruleCommentPkg, f.Path, line, n, msg)}
}

// ratioFinding is comment-ratio: a file's comment lines no placement rule flagged, over
// CommentRatio percent of its non-blank lines, judged once the file has at least
// commentRatioMinLines of them. doc.go is all comment by design: comment-pkg judges it.
func ratioFinding(ctx *lane.Context, f *gosrc.File, flagged map[*ast.CommentGroup]bool) []finding.Finding {
	if !ctx.On(ruleCommentRatio) || isDocGo(f) {
		return nil
	}
	nonBlank := countNonBlank(f.Src)
	if nonBlank < commentRatioMinLines {
		return nil
	}
	pct := unflaggedCommentLines(f.AST, flagged) * ratioPercentScale / nonBlank
	if pct <= rules.CommentRatio {
		return nil
	}
	msg := fmt.Sprintf(fmtMeasuredPercent, pct, rules.CommentRatio)
	return []finding.Finding{sizeFinding(ruleCommentRatio, f.Path, 0, pct, msg)}
}

func unflaggedCommentLines(file *ast.File, flagged map[*ast.CommentGroup]bool) int {
	n := 0
	for _, g := range file.Comments {
		if flagged[g] {
			continue
		}
		for _, c := range g.List {
			n += commentTextLines(c)
		}
	}
	return n
}
