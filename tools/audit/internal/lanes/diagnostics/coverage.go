package diagnostics

import (
	"fmt"
	"go/ast"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/tools/audit/internal/finding"
)

// site is where a code is first reported.
type site struct {
	file string
	line int
}

// coverage judges every catalogued code: reported by no compiler code (unreported), or
// reported with no per-code test producing it (untested).
func (s *scan) coverage() {
	reported := s.goReferences()
	s.runtimeReferences(reported)
	for _, e := range s.cat.codes {
		at, ok := reported[e.code]
		switch {
		case !ok:
			s.emit(finding.Finding{
				Rule: ruleUnreported, File: catalogueFile, Line: e.line, Symbol: e.code,
				Message: fmt.Sprintf(msgUnreported, e.code, e.pkg),
			})
		case !s.tested(e):
			s.emit(finding.Finding{
				Rule: ruleUntested, File: at.file, Line: at.line, Symbol: e.code,
				Message: fmt.Sprintf(msgUntested, e.code, findingsDirOf(e.pkg), e.code),
			})
		}
	}
}

// goReferences maps each code to its first mention, diag.E3501, in non-test Go code outside
// internal/diag: a code production code names is a code the compiler can report.
func (s *scan) goReferences() map[string]site {
	out := map[string]site{}
	for _, f := range s.files {
		if f.TestCode() {
			continue
		}
		names := s.diagNames(f.AST)
		ast.Inspect(f.AST, func(n ast.Node) bool {
			code, ok := codeRef(n, names)
			if _, seen := out[code]; ok && !seen {
				out[code] = site{file: f.Path, line: s.ctx.Go.Line(n.Pos())}
			}
			return true
		})
	}
	return out
}

// diagNames are the names the file imports the registry under ("." for a dot import).
func (s *scan) diagNames(file *ast.File) map[string]bool {
	names := map[string]bool{}
	for _, is := range file.Imports {
		if p, err := strconv.Unquote(is.Path.Value); err != nil || p != s.diagPath {
			continue
		}
		name := filepath.Base(diagDir)
		if is.Name != nil {
			name = is.Name.Name
		}
		names[name] = true
	}
	return names
}

// codeRef is the code n names through the registry: diag.E3501, or E3501 under a dot import.
func codeRef(n ast.Node, names map[string]bool) (string, bool) {
	switch n := n.(type) {
	case *ast.SelectorExpr:
		id, ok := n.X.(*ast.Ident)
		if ok && id.Obj == nil && names[id.Name] && reCode.MatchString(n.Sel.Name) {
			return n.Sel.Name, true
		}
	case *ast.Ident:
		if names[dotImport] && n.Obj == nil && reCode.MatchString(n.Name) {
			return n.Name, true
		}
	}
	return "", false
}

// runtimeReferences adds the codes the runtime helper texts signal: generated code reports them.
func (s *scan) runtimeReferences(reported map[string]site) {
	paths, _ := filepath.Glob(s.ctx.Repo.Abs(runtimeTexts))
	for _, p := range paths {
		src, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		rel, _ := filepath.Rel(s.ctx.Repo.Root, p)
		for i, line := range strings.Split(string(src), lineBreak) {
			addCodes(reported, line, site{file: filepath.ToSlash(rel), line: i + 1})
		}
	}
}

// addCodes records every code token of line at at, unless an earlier site has it.
func addCodes(reported map[string]site, line string, at site) {
	for _, m := range reCodeToken.FindAllStringSubmatch(line, allMatches) {
		if _, seen := reported[m[codeGroup]]; !seen {
			reported[m[codeGroup]] = at
		}
	}
}

// tested reports a per-code test of e in its owning package: a <CODE>_<n>.txtar whose
// findings.txt section holds the code.
func (s *scan) tested(e entry) bool {
	paths, _ := filepath.Glob(s.ctx.Repo.Abs(findingsDirOf(e.pkg) + pathSep + e.code + txtarGlob))
	for _, p := range paths {
		m := reTxtarName.FindStringSubmatch(filepath.Base(p))
		if m == nil || m[codeGroup] != e.code {
			continue
		}
		if src, err := os.ReadFile(p); err == nil && holdsCode(findingsText(string(src)), e.code) {
			return true
		}
	}
	return false
}

// findingsText is the findings.txt section of a txtar archive.
func findingsText(src string) string {
	var b strings.Builder
	in := false
	for line := range strings.SplitSeq(src, lineBreak) {
		if reTxtarMarker.MatchString(line) {
			in = line == findingsSection
			continue
		}
		if in {
			b.WriteString(line + lineBreak)
		}
	}
	return b.String()
}

func holdsCode(text, code string) bool {
	for _, m := range reCodeToken.FindAllStringSubmatch(text, allMatches) {
		if m[codeGroup] == code {
			return true
		}
	}
	return false
}
