package stock

import (
	"go/ast"
	"slices"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/tools/audit/internal/gosrc"
)

// dropIssue reports an issue its rule lets stand: anything but formatting in harness code,
// contextcheck in request code (ctx comes from r.Context()), wrapcheck in a rows-scan
// callback (its caller wraps).
func dropIssue(tree *gosrc.Tree, file, rule string, iss golangciIssue) bool {
	if tree == nil {
		return false
	}
	f, ok := tree.File(file)
	if !ok {
		return false
	}
	switch {
	case f.Harness:
		return rule != ruleFmt
	case iss.FromLinter == linterContextcheck:
		return requestCode(tree, f, iss)
	case iss.FromLinter == linterWrapcheck:
		fns := enclosingFuncs(tree, f, iss.Pos.Line)
		return len(fns) > 0 && rowScanner(f.AST, fns[len(fns)-1])
	}
	return false
}

// requestCode reports a contextcheck issue in a function taking a *http.Request, or on a call
// of one.
func requestCode(tree *gosrc.Tree, f *gosrc.File, iss golangciIssue) bool {
	if slices.ContainsFunc(enclosingFuncs(tree, f, iss.Pos.Line), func(ft *ast.FuncType) bool {
		return takesRequest(f.AST, ft)
	}) {
		return true
	}
	chain := ctxChain(iss.Text)
	return len(chain) > 0 && calleeTakesRequest(tree, chain[0])
}

// ctxChain is the call chain of a "Function `a->b` should pass the context parameter" issue,
// closure suffixes ($1) dropped; nil for any other contextcheck text.
func ctxChain(text string) []string {
	_, rest, ok := strings.Cut(text, ctxChainOpen)
	chain, _, ok2 := strings.Cut(rest, ctxChainClose)
	if !ok || !ok2 || !strings.Contains(text, ctxPassMarker) {
		return nil
	}
	out := strings.Split(chain, ctxChainSep)
	for i, name := range out {
		name, _, _ = strings.Cut(name, closureMark)
		out[i] = name
	}
	return out
}

// calleeTakesRequest reports a function of the repo named name taking a *http.Request.
func calleeTakesRequest(tree *gosrc.Tree, name string) bool {
	return slices.ContainsFunc(tree.Files, func(f *gosrc.File) bool {
		return slices.ContainsFunc(f.AST.Decls, func(d ast.Decl) bool {
			fd, ok := d.(*ast.FuncDecl)
			return ok && fd.Name.Name == name && takesRequest(f.AST, fd.Type)
		})
	})
}

// enclosingFuncs lists the function types around line, outermost first.
func enclosingFuncs(tree *gosrc.Tree, f *gosrc.File, line int) []*ast.FuncType {
	var out []*ast.FuncType
	ast.Inspect(f.AST, func(n ast.Node) bool {
		if n == nil || tree.Line(n.Pos()) > line || tree.Line(n.End()) < line {
			return n == nil
		}
		switch fn := n.(type) {
		case *ast.FuncDecl:
			out = append(out, fn.Type)
		case *ast.FuncLit:
			out = append(out, fn.Type)
		}
		return true
	})
	return out
}

func takesRequest(file *ast.File, ft *ast.FuncType) bool {
	return ft.Params != nil && slices.ContainsFunc(ft.Params.List, func(p *ast.Field) bool {
		return pointerTo(file, p.Type, pkgNetHTTP, typeRequest)
	})
}

// rowScanner is func(*sql.Rows) (T, error): a scan callback whose caller wraps the error.
func rowScanner(file *ast.File, ft *ast.FuncType) bool {
	if ft.Params == nil || len(ft.Params.List) != 1 || len(ft.Params.List[0].Names) > 1 {
		return false
	}
	if ft.Results == nil || len(ft.Results.List) != scanResults || len(ft.Results.List[1].Names) > 1 {
		return false
	}
	last, ok := ft.Results.List[1].Type.(*ast.Ident)
	return ok && last.Name == typeError && pointerTo(file, ft.Params.List[0].Type, pkgSQL, typeRows)
}

// pointerTo reports x spelling *<pkg>.<name>, the package resolved through file's imports.
func pointerTo(file *ast.File, x ast.Expr, pkg, name string) bool {
	star, ok := x.(*ast.StarExpr)
	if !ok {
		return false
	}
	sel, ok := star.X.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != name {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && importedAs(file, id.Name) == pkg
}

func importedAs(file *ast.File, name string) string {
	for _, is := range file.Imports {
		p, err := strconv.Unquote(is.Path.Value)
		if err != nil {
			continue
		}
		local := p[strings.LastIndex(p, "/")+1:]
		if is.Name != nil {
			local = is.Name.Name
		}
		if local == name {
			return p
		}
	}
	return ""
}
