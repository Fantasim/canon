package gorules

import (
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/fantasim/canonlang/tools/audit/internal/gosrc"
)

// nodeSpan locates a node across file sets: the typed load parses its own copy of each file.
type nodeSpan struct {
	file     string
	from, to int
}

// quietIndex holds the spans whose strings magic-string does not judge: logger arguments,
// map keys and indexes, route patterns, as the type-checked load resolves them.
type quietIndex struct {
	fset  *token.FileSet
	spans map[nodeSpan]bool
	typed bool
}

// newQuietIndex resolves quiet spans from the typed load; when it fails, only syntax decides.
func newQuietIndex(tree *gosrc.Tree) *quietIndex {
	q := &quietIndex{fset: tree.Fset, spans: map[nodeSpan]bool{}}
	pkgs, err := tree.Typed()
	if err != nil {
		return q
	}
	for _, p := range pkgs {
		if p.TypesInfo == nil || p.Fset == nil {
			continue
		}
		q.typed = true
		for _, file := range p.Syntax {
			ast.Inspect(file, func(n ast.Node) bool {
				q.typedNode(p, n)
				return true
			})
		}
	}
	return q
}

func (q *quietIndex) typedNode(p *packages.Package, n ast.Node) {
	switch n := n.(type) {
	case *ast.CallExpr:
		fn := gosrc.CalledFunc(p.TypesInfo, n)
		if fn == nil {
			return
		}
		if loggerFunc(fn) {
			q.addAll(p.Fset, n.Args)
		}
		if routeMethod(fn) {
			q.addAll(p.Fset, patternArg(fn.Name(), n.Args))
		}
	case *ast.CompositeLit:
		if isMap(p.TypesInfo.TypeOf(n)) {
			q.addAll(p.Fset, mapKeys(n))
		}
	case *ast.IndexExpr:
		if isMap(p.TypesInfo.TypeOf(n.X)) {
			q.addAll(p.Fset, []ast.Expr{n.Index})
		}
	}
}

func (q *quietIndex) addAll(fset *token.FileSet, xs []ast.Expr) {
	for _, x := range xs {
		q.spans[spanOf(fset, x)] = true
	}
}

// has reports a node of the parsed tree the typed load marked quiet.
func (q *quietIndex) has(n ast.Node) bool {
	return q != nil && len(q.spans) > 0 && q.spans[spanOf(q.fset, n)]
}

func spanOf(fset *token.FileSet, n ast.Node) nodeSpan {
	from, to := fset.Position(n.Pos()), fset.Position(n.End())
	return nodeSpan{file: filepath.Clean(from.Filename), from: from.Offset, to: to.Offset}
}

// loggerFunc is a function of log or log/slog, or a method of their loggers.
func loggerFunc(fn *types.Func) bool {
	return fn.Pkg() != nil && slices.Contains(logPkgs, fn.Pkg().Path())
}

// routeMethod is a route registration method of a router or mux type.
func routeMethod(fn *types.Func) bool {
	recv := fn.Signature().Recv()
	if recv == nil || !slices.Contains(routeFuncs, fn.Name()) {
		return false
	}
	t := recv.Type()
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	named, ok := t.(*types.Named)
	return ok && routerType(named.Obj().Name())
}

func routerType(name string) bool {
	return slices.ContainsFunc(routerTypeMarks, func(m string) bool { return strings.Contains(name, m) })
}

// patternArg is the route pattern among a registration call's arguments.
func patternArg(name string, args []ast.Expr) []ast.Expr {
	i := firstArg
	if name == fnMethod {
		i = secondArg
	}
	if i >= len(args) {
		return nil
	}
	return args[i : i+1]
}

func isMap(t types.Type) bool {
	if t == nil {
		return false
	}
	_, ok := t.Underlying().(*types.Map)
	return ok
}

func mapKeys(c *ast.CompositeLit) []ast.Expr {
	var out []ast.Expr
	for _, e := range c.Elts {
		if kv, ok := e.(*ast.KeyValueExpr); ok {
			out = append(out, kv.Key)
		}
	}
	return out
}

// quietSyntax marks, on the parsed tree, what syntax alone shows magic-string does not
// judge; without a typed load it also trusts logger and router method names.
func (f *src) quietSyntax(n ast.Node, q *quietIndex, quiet map[ast.Node]bool) {
	switch n := n.(type) {
	case *ast.CallExpr:
		if pkg, _ := f.callee(n); slices.Contains(logPkgs, pkg) {
			markAll(quiet, n.Args)
		}
		if q == nil || !q.typed {
			f.quietByName(n, quiet)
		}
	case *ast.CompositeLit:
		if _, ok := n.Type.(*ast.MapType); ok {
			markAll(quiet, mapKeys(n))
		}
	}
	if q.has(n) {
		quiet[n] = true
	}
}

// quietByName guesses a logger call from its method name and a receiver spelled like a
// logger, and a route registration from its method name and a "/" pattern.
func (f *src) quietByName(c *ast.CallExpr, quiet map[ast.Node]bool) {
	sel, ok := c.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	recv := strings.ToLower(types.ExprString(sel.X))
	if slices.Contains(loggerMethods, sel.Sel.Name) && strings.Contains(recv, logMark) {
		markAll(quiet, c.Args)
	}
	if !slices.Contains(routeFuncs, sel.Sel.Name) {
		return
	}
	for _, a := range patternArg(sel.Sel.Name, c.Args) {
		if v, ok := unquote(a); ok && strings.Contains(v, pathSep) {
			quiet[a] = true
		}
	}
}

func markAll(quiet map[ast.Node]bool, xs []ast.Expr) {
	for _, x := range xs {
		quiet[x] = true
	}
}

// quietNumber reports a number the magic-number rule lets stand: a shift count, or a count
// of a time unit (24 * time.Hour).
func (f *src) quietNumber(lit *ast.BasicLit, stack []ast.Node) bool {
	child := ast.Node(lit)
	for _, n := range slices.Backward(stack) {
		quiet, climb := f.numberParent(n, child)
		if !climb {
			return quiet
		}
		child = n
	}
	return false
}

// numberParent judges one ancestor of a number: quiet decides it, climb asks its parent.
func (f *src) numberParent(n, child ast.Node) (quiet, climb bool) {
	switch p := n.(type) {
	case *ast.ParenExpr:
		return false, true
	case *ast.CallExpr:
		return false, f.duration(p)
	case *ast.AssignStmt:
		return p.Tok == token.SHL_ASSIGN || p.Tok == token.SHR_ASSIGN, false
	case *ast.BinaryExpr:
		if shift := p.Op == token.SHL || p.Op == token.SHR; shift || p.Op != token.MUL {
			return shift && p.Y == child, false
		}
		return f.duration(p.X) || f.duration(p.Y), !f.duration(p.X) && !f.duration(p.Y)
	}
	return false, false
}

// duration reports a time unit (time.Hour) or a time.Duration conversion.
func (f *src) duration(x ast.Expr) bool {
	units := timeUnits
	x = ast.Unparen(x)
	if c, ok := x.(*ast.CallExpr); ok {
		x, units = c.Fun, []string{typeDuration}
	}
	sel, ok := x.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Obj == nil && f.imports[id.Name] == pkgTime && slices.Contains(units, sel.Sel.Name)
}
