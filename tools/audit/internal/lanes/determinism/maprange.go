package determinism

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/fantasim/canonlang/tools/audit/internal/finding"
	"github.com/fantasim/canonlang/tools/audit/internal/gosrc"
	"github.com/fantasim/canonlang/tools/audit/internal/lane"
)

// scan is one run: the hand-written files by absolute path, those already judged, findings.
type scan struct {
	ctx    *lane.Context
	byAbs  map[string]*gosrc.File
	judged map[string]bool
	out    []finding.Finding
}

// mark is one //canon:unordered comment: its line and reason.
type mark struct {
	line   int
	reason string
	used   bool
}

func newScan(ctx *lane.Context) *scan {
	s := &scan{ctx: ctx, byAbs: map[string]*gosrc.File{}, judged: map[string]bool{}}
	for _, f := range ctx.Go.Files {
		if !f.Generated {
			s.byAbs[filepath.Clean(f.Abs)] = f
		}
	}
	return s
}

// packages judges each file once: a file belongs to its package and to its test variants.
func (s *scan) packages(pkgs []*packages.Package) {
	for _, p := range pkgs {
		if p.TypesInfo == nil {
			continue
		}
		for _, file := range p.Syntax {
			gf, ok := s.byAbs[filepath.Clean(p.Fset.Position(file.Pos()).Filename)]
			if !ok || s.judged[gf.Path] {
				continue
			}
			s.judged[gf.Path] = true
			s.file(p, file, gf)
		}
	}
}

// file reports the unmarked map ranges of an output package, then every marker without a
// reason or without a map range on its line or the next.
func (s *scan) file(p *packages.Package, file *ast.File, gf *gosrc.File) {
	marks := marksOf(p.Fset, file)
	output := isOutput(gf.Dir)
	ast.Inspect(file, func(n ast.Node) bool {
		rs, ok := n.(*ast.RangeStmt)
		if !ok {
			return true
		}
		detail, msg := unordered(p.TypesInfo, rs.X)
		if detail == "" {
			return true
		}
		line := p.Fset.Position(rs.For).Line
		if m := markFor(marks, line); m != nil {
			m.used = true
		} else if output {
			s.emit(gf, line, detail, msg)
		}
		return true
	})
	for _, m := range marks {
		switch {
		case !m.used:
			s.emit(gf, m.line, detailStale, msgStale)
		case m.reason == "":
			s.emit(gf, m.line, detailReason, msgReason)
		}
	}
}

// unordered names what makes ranging over x follow map order, "" when nothing does.
func unordered(info *types.Info, x ast.Expr) (string, string) {
	if t := info.TypeOf(x); t != nil {
		if _, ok := t.Underlying().(*types.Map); ok {
			return detailRange, fmt.Sprintf(msgRange, t)
		}
	}
	call, ok := ast.Unparen(x).(*ast.CallExpr)
	if !ok {
		return "", ""
	}
	fn := gosrc.CalledFunc(info, call)
	if fn == nil || fn.Pkg() == nil || fn.Pkg().Path() != pkgMaps || !slices.Contains(mapIterators, fn.Name()) {
		return "", ""
	}
	return detailIterator, fmt.Sprintf(msgIterator, fn.Name())
}

// marksOf is every marker of the file, in line order.
func marksOf(fset *token.FileSet, file *ast.File) []*mark {
	var out []*mark
	for _, g := range file.Comments {
		for _, c := range g.List {
			rest, ok := strings.CutPrefix(c.Text, marker)
			if !ok || rest != "" && rest[0] != ' ' && rest[0] != '\t' {
				continue
			}
			out = append(out, &mark{line: fset.Position(c.Pos()).Line, reason: strings.TrimSpace(rest)})
		}
	}
	return out
}

// markFor is the marker on the range's line, else on the line above; nil for none.
func markFor(marks []*mark, line int) *mark {
	for _, want := range []int{line, line - 1} {
		if i := slices.IndexFunc(marks, func(m *mark) bool { return m.line == want }); i >= 0 {
			return marks[i]
		}
	}
	return nil
}

// isOutput reports a directory of an output package or below one.
func isOutput(dir string) bool {
	return slices.ContainsFunc(outputDirs, func(d string) bool { return dir == d || strings.HasPrefix(dir, d+pathSep) })
}

func (s *scan) emit(gf *gosrc.File, line int, detail, msg string) {
	s.out = append(s.out, finding.Finding{Rule: ruleMapRange, File: gf.Path, Line: line, Detail: detail, Message: msg})
}
