package gorules

import (
	"go/ast"
	"go/token"
	"slices"
)

// constDecl is one const spec name with the expression that gives it its value.
type constDecl struct {
	file  *src
	spec  *ast.ValueSpec
	decl  *ast.GenDecl
	name  *ast.Ident
	value ast.Expr
	local bool
}

// evaluator folds string constant expressions of one package: literals, concatenations,
// same-package const names and conversions.
type evaluator struct {
	consts map[string]ast.Expr
	busy   map[string]bool
}

func newEvaluator() *evaluator {
	return &evaluator{consts: map[string]ast.Expr{}, busy: map[string]bool{}}
}

func (e *evaluator) str(x ast.Expr) (string, bool) {
	switch x := x.(type) {
	case *ast.BasicLit:
		return unquote(x)
	case *ast.ParenExpr:
		return e.str(x.X)
	case *ast.BinaryExpr:
		if x.Op != token.ADD {
			return "", false
		}
		a, ok := e.str(x.X)
		if !ok {
			return "", false
		}
		b, ok := e.str(x.Y)
		return a + b, ok
	case *ast.Ident:
		return e.ident(x.Name)
	case *ast.CallExpr:
		if len(x.Args) == 1 && isTypeName(x.Fun) {
			return e.str(x.Args[firstArg])
		}
	}
	return "", false
}

// sqlText folds a string concatenation as str does, standing sqlOperand in for an operand
// it cannot fold (another package's constant: config.PGNowUTC), so the statement's own
// words still decide what it is.
func (e *evaluator) sqlText(x ast.Expr) (string, bool) {
	if v, ok := e.str(x); ok {
		return v, true
	}
	switch x := x.(type) {
	case *ast.ParenExpr:
		return e.sqlText(x.X)
	case *ast.BinaryExpr:
		if x.Op != token.ADD {
			return "", false
		}
		a, ok := e.sqlText(x.X)
		if !ok {
			return "", false
		}
		b, ok := e.sqlText(x.Y)
		return a + b, ok
	case *ast.Ident, *ast.SelectorExpr:
		return sqlOperand, true
	}
	return "", false
}

func (e *evaluator) ident(name string) (string, bool) {
	x, ok := e.consts[name]
	if !ok || e.busy[name] {
		return "", false
	}
	e.busy[name] = true
	defer delete(e.busy, name)
	return e.str(x)
}

func isTypeName(x ast.Expr) bool {
	switch x := x.(type) {
	case *ast.Ident:
		return true
	case *ast.SelectorExpr:
		_, ok := x.X.(*ast.Ident)
		return ok
	}
	return false
}

// alias reports a const whose value is only another name: a deliberate second name.
func alias(x ast.Expr) bool {
	switch x := x.(type) {
	case *ast.Ident:
		return true
	case *ast.SelectorExpr:
		return true
	case *ast.ParenExpr:
		return alias(x.X)
	}
	return false
}

// constDecls lists every const spec name of the files, top-level and local, with values.
func constDecls(files []*src) []constDecl {
	var out []constDecl
	for _, f := range files {
		ast.Inspect(f.AST, func(n ast.Node) bool {
			g, ok := n.(*ast.GenDecl)
			if !ok || g.Tok != token.CONST {
				return true
			}
			out = append(out, specDecls(f, g, !slices.Contains(f.AST.Decls, ast.Decl(g)))...)
			return false
		})
	}
	return out
}

func specDecls(f *src, g *ast.GenDecl, local bool) []constDecl {
	var out []constDecl
	for _, sp := range g.Specs {
		vs, ok := sp.(*ast.ValueSpec)
		if !ok {
			continue
		}
		for i, name := range vs.Names {
			if i < len(vs.Values) && name.Name != underscore {
				out = append(out, constDecl{file: f, spec: vs, decl: g, name: name, value: vs.Values[i], local: local})
			}
		}
	}
	return out
}

// evaluators builds one evaluator per package directory from its top-level consts.
func evaluators(decls []constDecl) map[string]*evaluator {
	out := map[string]*evaluator{}
	for _, d := range decls {
		ev, ok := out[d.file.Dir]
		if !ok {
			ev = newEvaluator()
			out[d.file.Dir] = ev
		}
		if !d.local {
			ev.consts[d.name.Name] = d.value
		}
	}
	return out
}
