package gostyle

import (
	"fmt"
	"go/ast"

	"github.com/fantasim/canonlang/tools/audit/internal/finding"
	"github.com/fantasim/canonlang/tools/audit/internal/gosrc"
	"github.com/fantasim/canonlang/tools/audit/internal/lane"
)

// sizeFindings runs every size rule over the parsed tree.
func sizeFindings(ctx *lane.Context) []finding.Finding {
	var out []finding.Finding
	for _, f := range ctx.Go.Files {
		if f.Generated {
			continue
		}
		out = append(out, fileSizeFindings(ctx, f)...)
	}
	return append(out, pkgSizeFindings(ctx)...)
}

func fileSizeFindings(ctx *lane.Context, f *gosrc.File) []finding.Finding {
	var out []finding.Finding
	if ctx.On(ruleFileLength) && !f.TestCode() {
		if n := countLines(f.Src); n > ctx.Limits.FileLines {
			out = append(out, sizeFinding(ruleFileLength, f.Path, 0, n, measured(n, unitLines, ctx.Limits.FileLines)))
		}
	}
	for _, d := range f.AST.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Body != nil {
			out = append(out, funcFindings(ctx, f, fd)...)
		}
	}
	return out
}

// signatureFindings checks the parameter and result counts.
func signatureFindings(ctx *lane.Context, f *gosrc.File, fd *ast.FuncDecl, line int) []finding.Finding {
	var out []finding.Finding
	if n := fieldListCount(fd.Type.Params); ctx.On(ruleFnParams) && n > ctx.Limits.FnParams {
		out = append(out, sizeFinding(ruleFnParams, f.Path, line, n, measured(n, unitParams, ctx.Limits.FnParams)))
	}
	if n := fieldListCount(fd.Type.Results); ctx.On(ruleFnResults) && n > ctx.Limits.FnResults {
		out = append(out, sizeFinding(ruleFnResults, f.Path, line, n, measured(n, unitResults, ctx.Limits.FnResults)))
	}
	return out
}

// funcFindings runs the per-function size rules; a test file's method is exempt from the
// signature limits, since a mock mirrors the interface it stands in for.
func funcFindings(ctx *lane.Context, f *gosrc.File, fd *ast.FuncDecl) []finding.Finding {
	var out []finding.Finding
	line := ctx.Go.Line(fd.Pos())
	if ctx.On(ruleFnLength) {
		if fx, ok := fnLengthFinding(ctx, f, fd, line); ok {
			out = append(out, fx)
		}
	}
	if !f.Test || fd.Recv == nil {
		out = append(out, signatureFindings(ctx, f, fd, line)...)
	}
	if ctx.On(ruleFnNesting) {
		if n := maxNesting(fd.Body); n > ctx.Limits.FnNesting {
			out = append(out, sizeFinding(ruleFnNesting, f.Path, line, n, measured(n, unitLevels, ctx.Limits.FnNesting)))
		}
	}
	if ctx.On(ruleNakedReturn) {
		if fx, ok := nakedReturnFinding(ctx, f, fd, line); ok {
			out = append(out, fx)
		}
	}
	return out
}

// fnLengthFinding checks the body-lines limit first (tests use TestFnLines), then, for
// non-test functions only, the statement-count limit.
func fnLengthFinding(ctx *lane.Context, f *gosrc.File, fd *ast.FuncDecl, line int) (finding.Finding, bool) {
	bodyLines := ctx.Go.Lines(fd.Body)
	limit := ctx.Limits.FnLines
	if f.TestCode() {
		limit = ctx.Limits.TestFnLines
	}
	if bodyLines > limit {
		return sizeFinding(ruleFnLength, f.Path, line, bodyLines, measured(bodyLines, unitLines, limit)), true
	}
	if !f.TestCode() {
		if n := countStatements(fd.Body); n > ctx.Limits.FnStatements {
			msg := measured(n, unitStatements, ctx.Limits.FnStatements)
			return sizeFinding(ruleFnLength, f.Path, line, bodyLines, msg), true
		}
	}
	return finding.Finding{}, false
}

// countStatements counts every statement reachable from body, excluding the blocks
// themselves (a block's own List members are what get counted).
func countStatements(body *ast.BlockStmt) int {
	n := 0
	ast.Inspect(body, func(node ast.Node) bool {
		switch node.(type) {
		case *ast.BlockStmt:
		case ast.Stmt:
			n++
		}
		return true
	})
	return n
}

func nakedReturnFinding(ctx *lane.Context, f *gosrc.File, fd *ast.FuncDecl, line int) (finding.Finding, bool) {
	if !namedResults(fd.Type) {
		return finding.Finding{}, false
	}
	bodyLines := ctx.Go.Lines(fd.Body)
	if bodyLines <= ctx.Limits.NakedReturnLines {
		return finding.Finding{}, false
	}
	n := countNakedReturns(fd.Body)
	if n == 0 {
		return finding.Finding{}, false
	}
	msg := fmt.Sprintf(fmtNakedReturn, n, bodyLines, ctx.Limits.NakedReturnLines)
	return sizeFinding(ruleNakedReturn, f.Path, line, n, msg), true
}

// countNakedReturns counts bare `return` statements in body, not descending into a
// closure: a naked return there returns from the closure, not this function.
func countNakedReturns(body *ast.BlockStmt) int {
	n := 0
	ast.Inspect(body, func(node ast.Node) bool {
		if _, ok := node.(*ast.FuncLit); ok {
			return false
		}
		if rs, ok := node.(*ast.ReturnStmt); ok && len(rs.Results) == 0 {
			n++
		}
		return true
	})
	return n
}

func pkgSizeFindings(ctx *lane.Context) []finding.Finding {
	if !ctx.On(rulePkgSize) {
		return nil
	}
	var out []finding.Finding
	for dir, files := range ctx.Go.Packages() {
		nFiles, nLines := 0, 0
		for _, f := range files {
			if f.TestCode() {
				continue
			}
			nFiles++
			nLines += countLines(f.Src)
		}
		if nFiles > ctx.Limits.PkgFiles || nLines > ctx.Limits.PkgLines {
			msg := fmt.Sprintf(fmtPkgSize, nFiles, nLines, ctx.Limits.PkgFiles, ctx.Limits.PkgLines)
			out = append(out, sizeFinding(rulePkgSize, dir, 0, nLines, msg))
		}
	}
	return out
}
