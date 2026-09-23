package stock

import (
	"go/ast"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/tools/audit/internal/finding"
	"github.com/fantasim/canonlang/tools/audit/internal/gosrc"
	"github.com/fantasim/canonlang/tools/audit/internal/lane"
	"github.com/fantasim/canonlang/tools/audit/internal/repo"
)

type deadEntry struct {
	file string
	line int
	name string
}

// runDeadcode runs `deadcode -test <package dirs>` once for dead-unreachable and dead-file. A nil,
// nil result means neither rule is on, not a failure.
func runDeadcode(ctx *lane.Context) ([]finding.Finding, *lane.Skip) {
	if !ctx.On(ruleDeadUnreachable) && !ctx.On(ruleDeadFile) {
		return nil, nil
	}
	targets := goDirs(ctx.Repo.FilesWithExt(repo.GoExt))
	if len(targets) == 0 {
		return nil, nil
	}
	bin, err := resolveTool(ctx.Toolchain, toolDeadcode)
	if err != nil {
		return nil, &lane.Skip{What: skipDeadcode, Reason: err.Error()}
	}
	out, err := runTool(bin, ctx.Repo.Root, append([]string{argDeadcodeTest}, targets...)...)
	if err != nil {
		return nil, &lane.Skip{What: skipDeadcode, Reason: err.Error()}
	}
	entries := parseDeadcode(string(out))
	var findings []finding.Finding
	if ctx.On(ruleDeadUnreachable) {
		findings = append(findings, unreachableFindings(entries)...)
	}
	if ctx.On(ruleDeadFile) && ctx.Go != nil {
		findings = append(findings, deadFileFindings(ctx.Go, entries)...)
	}
	return findings, nil
}

// parseDeadcode reads deadcode's default output: "path:line:col: unreachable func: Name".
func parseDeadcode(out string) []deadEntry {
	var entries []deadEntry
	for l := range strings.SplitSeq(out, "\n") {
		m := reUnreachableFunc.FindStringSubmatch(l)
		if m == nil {
			continue
		}
		n, _ := strconv.Atoi(m[unreachableLineGroup])
		entries = append(entries, deadEntry{file: filepath.ToSlash(m[1]), line: n, name: m[unreachableNameGroup]})
	}
	return entries
}

func unreachableFindings(entries []deadEntry) []finding.Finding {
	out := make([]finding.Finding, 0, len(entries))
	for _, e := range entries {
		out = append(out, finding.Finding{
			Rule:    ruleDeadUnreachable,
			File:    e.file,
			Line:    e.line,
			Detail:  finding.Normalize(e.name),
			Message: unreachableFuncPfx + e.name,
		})
	}
	return out
}

// deadFileFindings flags a non-test, non-generated file whose every top-level function is
// unreachable and that declares no type.
func deadFileFindings(tree *gosrc.Tree, entries []deadEntry) []finding.Finding {
	unreachable := unreachableSet(entries)
	var out []finding.Finding
	for _, f := range tree.Files {
		if f.TestCode() || f.Generated {
			continue
		}
		if cand, ok := deadFileCandidate(tree, f, unreachable); ok {
			out = append(out, cand)
		}
	}
	return out
}

func unreachableSet(entries []deadEntry) map[string]map[string]bool {
	set := map[string]map[string]bool{}
	for _, e := range entries {
		m, ok := set[e.file]
		if !ok {
			m = map[string]bool{}
			set[e.file] = m
		}
		m[e.name] = true
	}
	return set
}

func deadFileCandidate(tree *gosrc.Tree, f *gosrc.File, unreachable map[string]map[string]bool) (finding.Finding, bool) {
	funcs, hasType := fileDecls(f.AST)
	if hasType || len(funcs) == 0 || !allUnreachable(funcs, unreachable[f.Path]) {
		return finding.Finding{}, false
	}
	return finding.Finding{
		Rule:   ruleDeadFile,
		File:   f.Path,
		Line:   tree.Line(funcs[0].Pos()),
		Detail: deadFileDetail,
	}, true
}

func fileDecls(f *ast.File) (funcs []*ast.FuncDecl, hasType bool) {
	for _, d := range f.Decls {
		switch decl := d.(type) {
		case *ast.FuncDecl:
			funcs = append(funcs, decl)
		case *ast.GenDecl:
			if decl.Tok == token.TYPE {
				hasType = true
			}
		}
	}
	return funcs, hasType
}

func allUnreachable(funcs []*ast.FuncDecl, names map[string]bool) bool {
	if names == nil {
		return false
	}
	for _, fd := range funcs {
		if !names[gosrc.DeclName(fd)] {
			return false
		}
	}
	return true
}
