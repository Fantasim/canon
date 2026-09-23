package gorules

import (
	"go/ast"
	"go/token"
	"path"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/fantasim/canonlang/tools/audit/internal/finding"
	"github.com/fantasim/canonlang/tools/audit/internal/gosrc"
	"github.com/fantasim/canonlang/tools/audit/internal/lane"
)

// src is one non-generated file with its import table and the literals the magic rules judge.
type src struct {
	*gosrc.File
	imports map[string]string
	lits    []judged
}

// judged is a literal outside imports, consts, tags and error constructors, with its parent.
type judged struct {
	lit    *ast.BasicLit
	parent ast.Node
	// quietStr and quietNum exempt it from magic-string and magic-number respectively.
	quietStr, quietNum bool
}

type scan struct {
	ctx       *lane.Context
	files     []*src
	constList []constDecl
	evals     map[string]*evaluator
	out       []finding.Finding
}

func newScan(ctx *lane.Context) *scan {
	s := &scan{ctx: ctx}
	var q *quietIndex
	if ctx.On(ruleMagicString) {
		q = newQuietIndex(ctx.Go)
	}
	for _, f := range ctx.Go.Files {
		if f.Generated {
			continue
		}
		sf := &src{File: f, imports: importTable(f.AST)}
		sf.lits = sf.judgedLits(q)
		s.files = append(s.files, sf)
	}
	return s
}

// emit keeps a finding only when its rule is enabled.
func (s *scan) emit(f finding.Finding) {
	if s.ctx.On(f.Rule) {
		s.out = append(s.out, f)
	}
}

func (s *scan) line(p token.Pos) int { return s.ctx.Go.Line(p) }

// importPath is the import path of the package in repo-relative dir.
func (s *scan) importPath(dir string) string {
	if dir == dirHere {
		return s.ctx.Repo.Module
	}
	return s.ctx.Repo.Module + pathSep + dir
}

// source is the source text of n, whitespace folded.
func (s *scan) source(f *src, n ast.Node) string {
	from, to := s.ctx.Go.Fset.Position(n.Pos()).Offset, s.ctx.Go.Fset.Position(n.End()).Offset
	if from < 0 || to > len(f.Src) || from > to {
		return ""
	}
	return finding.Clean(string(f.Src[from:to]))
}

func importTable(f *ast.File) map[string]string {
	m := map[string]string{}
	for _, is := range f.Imports {
		p, err := strconv.Unquote(is.Path.Value)
		if err != nil {
			continue
		}
		name := path.Base(p)
		if is.Name != nil {
			name = is.Name.Name
		}
		m[name] = p
	}
	return m
}

// callee resolves a call of an imported package's function to (import path, name).
func (f *src) callee(call *ast.CallExpr) (string, string) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", ""
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok || id.Obj != nil {
		return "", ""
	}
	return f.imports[id.Name], sel.Sel.Name
}

func (f *src) calls(call *ast.CallExpr, pkg string, names ...string) bool {
	p, n := f.callee(call)
	return p == pkg && slices.Contains(names, n)
}

// errCtor reports a call of errors.New or fmt.Errorf.
func (f *src) errCtor(call *ast.CallExpr) bool {
	return f.calls(call, pkgErrors, fnNew) || f.calls(call, pkgFmt, fnErrorf)
}

func (f *src) base() string { return path.Base(f.Path) }

func (f *src) pkgName() string { return f.AST.Name.Name }

// judgedLits walks the file once, skipping what no magic rule judges.
func (f *src) judgedLits(q *quietIndex) []judged {
	var out []judged
	skip, quiet := map[ast.Node]bool{}, map[ast.Node]bool{}
	for _, d := range f.AST.Decls {
		if g, ok := d.(*ast.GenDecl); ok && f.unjudgedDecl(g, skip) {
			continue
		}
		var stack []ast.Node
		ast.Inspect(d, func(n ast.Node) bool {
			if n == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			f.quietSyntax(n, q, quiet)
			if !f.judge(n, skip, quiet, stack, &out) {
				return false
			}
			stack = append(stack, n)
			return true
		})
	}
	return out
}

// unjudgedDecl skips imports, consts and package-level vars in constants.go/errors.go, and
// marks literal-only var specs: named tables const-placement reports as a whole.
func (f *src) unjudgedDecl(g *ast.GenDecl, skip map[ast.Node]bool) bool {
	named := f.base() == constantsFile || f.base() == errorsFile
	if g.Tok == token.VAR && !named {
		for _, sp := range g.Specs {
			if vs, ok := sp.(*ast.ValueSpec); ok && f.literalSpec(vs) {
				skip[vs] = true
			}
		}
	}
	return g.Tok == token.IMPORT || g.Tok == token.CONST || g.Tok == token.VAR && named
}

func (f *src) judge(n ast.Node, skip, quiet map[ast.Node]bool, stack []ast.Node, out *[]judged) bool {
	if skip[n] {
		return false
	}
	switch n := n.(type) {
	case *ast.GenDecl:
		return n.Tok != token.CONST
	case *ast.Field:
		if n.Tag != nil {
			skip[n.Tag] = true
		}
	case *ast.CallExpr:
		f.skipArgs(n, skip)
	case *ast.BasicLit:
		*out = append(*out, f.judgedLit(n, stack, quiet))
	}
	return true
}

// judgedLit records lit with its parent and whether the magic rules let it stand.
func (f *src) judgedLit(lit *ast.BasicLit, stack []ast.Node, quiet map[ast.Node]bool) judged {
	j := judged{lit: lit, quietNum: f.quietNumber(lit, stack)}
	if len(stack) > 0 {
		j.parent = stack[len(stack)-1]
	}
	j.quietStr = quiet[lit] || slices.ContainsFunc(stack, func(n ast.Node) bool { return quiet[n] })
	return j
}

// skipArgs marks the call arguments no magic rule judges: the message of errors.New and
// fmt.Errorf (err-inline's), and the base and bit size numbers strconv takes.
func (f *src) skipArgs(c *ast.CallExpr, skip map[ast.Node]bool) {
	if f.errCtor(c) && len(c.Args) > 0 {
		skip[c.Args[firstArg]] = true
	}
	if pkg, _ := f.callee(c); pkg != pkgStrconv {
		return
	}
	for _, a := range c.Args {
		if l, ok := a.(*ast.BasicLit); ok && l.Kind == token.INT {
			skip[a] = true
		}
	}
}

// clip truncates s to at most n runes.
func clip(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return string(r[:n])
}

// underCmd reports a dir that is, or sits below, a cmd/ directory.
func underCmd(dir string) bool {
	return slices.Contains(strings.Split(dir, pathSep), cmdDirName)
}

// unquote returns the value of a string literal expression.
func unquote(e ast.Expr) (string, bool) {
	l, ok := e.(*ast.BasicLit)
	if !ok || l.Kind != token.STRING {
		return "", false
	}
	v, err := strconv.Unquote(l.Value)
	return v, err == nil
}
