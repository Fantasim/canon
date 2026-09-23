package gostyle

import (
	"go/ast"
	"strings"

	"github.com/fantasim/canonlang/tools/audit/internal/finding"
	"github.com/fantasim/canonlang/tools/audit/internal/gosrc"
	"github.com/fantasim/canonlang/tools/audit/internal/lane"
	"github.com/fantasim/canonlang/tools/audit/internal/rules"
)

// contentFindings scans every comment group in a file for content the placement rules
// don't judge: a narrated decision citation, change-history narration, or a task marker.
// A group a placement rule already flagged is not reported again for its citation.
func contentFindings(ctx *lane.Context, f *gosrc.File, flagged map[*ast.CommentGroup]bool) []finding.Finding {
	var out []finding.Finding
	for _, g := range f.AST.Comments {
		if !flagged[g] {
			out = append(out, adrFindings(ctx, f, g)...)
		}
		out = append(out, historyFindings(ctx, f, g)...)
		out = append(out, todoFindings(ctx, f, g)...)
	}
	return out
}

// adrFindings is comment-adr-narration: a decision-reference citation spanning multiple lines.
func adrFindings(ctx *lane.Context, f *gosrc.File, g *ast.CommentGroup) []finding.Finding {
	if !ctx.On(ruleADRNarration) || !reADRCite.MatchString(g.Text()) {
		return nil
	}
	n := contentLines(g)
	if n <= 1 {
		return nil
	}
	line := ctx.Go.Line(g.Pos())
	return []finding.Finding{sizeFinding(ruleADRNarration, f.Path, line, n, measured(n, unitLines, 1))}
}

// historyFindings is comment-history (observe-only): a comment describing earlier code.
func historyFindings(ctx *lane.Context, f *gosrc.File, g *ast.CommentGroup) []finding.Finding {
	if !ctx.On(ruleHistory) || !rules.NarratesHistory(g.Text()) {
		return nil
	}
	line := ctx.Go.Line(g.Pos())
	return []finding.Finding{sizeFinding(ruleHistory, f.Path, line, contentLines(g), "")}
}

// todoFindings is todo-in-code: one finding per physical comment line carrying an
// uppercase task marker (reTODO), Detail holding that normalized line.
func todoFindings(ctx *lane.Context, f *gosrc.File, g *ast.CommentGroup) []finding.Finding {
	if !ctx.On(ruleTODO) {
		return nil
	}
	var out []finding.Finding
	for _, c := range g.List {
		for i, ln := range strings.Split(c.Text, newline) {
			if reTODO.MatchString(ln) {
				fx := sizeFinding(ruleTODO, f.Path, ctx.Go.Line(c.Pos())+i, 1, "")
				fx.Detail = finding.Normalize(ln)
				out = append(out, fx)
			}
		}
	}
	return out
}
