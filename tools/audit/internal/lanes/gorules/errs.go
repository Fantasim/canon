package gorules

import (
	"fmt"
	"go/ast"
	"go/token"
	"path"
	"strings"

	"github.com/fantasim/canonlang/tools/audit/internal/finding"
)

// inFuncs calls visit on every call inside a function body of f: declared functions and
// function literals, package-level ones included.
func inFuncs(f *src, visit func(call *ast.CallExpr)) {
	calls := func(n ast.Node) bool {
		if c, ok := n.(*ast.CallExpr); ok {
			visit(c)
		}
		return true
	}
	for _, d := range f.AST.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok {
			if fd.Body != nil {
				ast.Inspect(fd.Body, calls)
			}
			continue
		}
		ast.Inspect(d, func(n ast.Node) bool {
			if fl, ok := n.(*ast.FuncLit); ok {
				ast.Inspect(fl.Body, calls)
				return false
			}
			return true
		})
	}
}

// errInline reports errors.New inside a function, and fmt.Errorf whose format lacks %w.
func (s *scan) errInline() {
	for _, f := range s.files {
		if f.TestCode() {
			continue
		}
		inFuncs(f, func(c *ast.CallExpr) { s.inlineErr(f, c) })
	}
}

func (s *scan) inlineErr(f *src, c *ast.CallExpr) {
	if len(c.Args) == 0 {
		return
	}
	arg := c.Args[firstArg]
	text, lit := unquote(arg)
	if !lit {
		text = s.source(f, arg)
	}
	target := path.Join(f.Dir, errorsFile)
	var msg, fix string
	switch {
	case f.calls(c, pkgErrors, fnNew):
		msg, fix = fmt.Sprintf(msgErrNew, clip(s.source(f, arg), snippetMax)), fmt.Sprintf(fixErrNew, target)
	case f.calls(c, pkgFmt, fnErrorf) && lit && !strings.Contains(text, wrapVerb):
		msg, fix = fmt.Sprintf(msgErrorf, clip(s.source(f, arg), snippetMax)), fixErrorf
	default:
		return
	}
	s.emit(finding.Finding{
		Rule: ruleErrInline, File: f.Path, Line: s.line(c.Pos()), Detail: finding.Normalize(text),
		Message: msg, Fix: fix,
	})
}

// errPlacement reports package-level error sentinels declared outside <pkg>/errors.go.
func (s *scan) errPlacement() {
	for _, f := range s.files {
		if f.TestCode() || f.base() == errorsFile {
			continue
		}
		for _, d := range f.AST.Decls {
			g, ok := d.(*ast.GenDecl)
			if !ok || g.Tok != token.VAR {
				continue
			}
			s.sentinels(f, g)
		}
	}
}

func (s *scan) sentinels(f *src, g *ast.GenDecl) {
	target := path.Join(f.Dir, errorsFile)
	for _, sp := range g.Specs {
		vs, ok := sp.(*ast.ValueSpec)
		if !ok {
			continue
		}
		for i, v := range vs.Values {
			c, ok := v.(*ast.CallExpr)
			if !ok || !f.errCtor(c) || i >= len(vs.Names) {
				continue
			}
			name := vs.Names[i]
			s.emit(finding.Finding{
				Rule: ruleErrPlace, File: f.Path, Line: s.line(name.Pos()), Detail: name.Name,
				Message: fmt.Sprintf(msgErrPlace, name.Name, target), Fix: fmt.Sprintf(fixMoveTo, target),
			})
		}
	}
}

// envKey reports environment access outside cmd/, package main or a config package, or with
// a literal key.
func (s *scan) envKey() {
	for _, f := range s.files {
		if f.TestCode() {
			continue
		}
		ast.Inspect(f.AST, func(n ast.Node) bool {
			if c, ok := n.(*ast.CallExpr); ok && f.calls(c, pkgOS, envFuncs...) {
				s.envCall(f, c)
			}
			return true
		})
	}
}

func (s *scan) envCall(f *src, c *ast.CallExpr) {
	_, fn := f.callee(c)
	key, lit := "", false
	if len(c.Args) > 0 {
		key, lit = unquote(c.Args[firstArg])
		if !lit {
			key = s.source(f, c.Args[firstArg])
		}
	}
	outside := path.Base(f.Dir) != configDirName && !underCmd(f.Dir) && f.pkgName() != mainPkg
	if !outside && !lit {
		return
	}
	fd := finding.Finding{Rule: ruleEnvKey, File: f.Path, Line: s.line(c.Pos()), Detail: fn + " " + key}
	switch {
	case outside:
		fd.Message, fd.Fix = fmt.Sprintf(msgEnvOutside, fn), fixEnvOutside
	default:
		fd.Message, fd.Fix = fmt.Sprintf(msgEnvLiteral, fn, key), fmt.Sprintf(fixEnvLiteral, path.Join(f.Dir, constantsFile))
	}
	s.emit(fd)
}
