package fnsize

import (
	"go/ast"
	gotoken "go/token"
)

// goFunc measures a Go body by the lines of its braces and the statements in its list.
func goFunc(fset *gotoken.FileSet, name string, body *ast.BlockStmt) Func {
	return Func{
		Name:       name,
		Body:       max(0, fset.Position(body.Rbrace).Line-fset.Position(body.Lbrace).Line-1),
		Statements: len(body.List),
	}
}

// funcName is Name for a function and Receiver.Name for a method, without pointer or type
// arguments on the receiver.
func funcName(d *ast.FuncDecl) string {
	if d.Recv == nil || len(d.Recv.List) == 0 {
		return d.Name.Name
	}
	return receiverName(d.Recv.List[0].Type) + "." + d.Name.Name
}

func receiverName(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.StarExpr:
		return receiverName(t.X)
	case *ast.IndexExpr:
		return receiverName(t.X)
	case *ast.IndexListExpr:
		return receiverName(t.X)
	case *ast.Ident:
		return t.Name
	}
	return anonymousName
}

// varLiterals measures the outermost function literals of a package-level declaration, named
// after the variable that holds them.
func varLiterals(fset *gotoken.FileSet, d *ast.GenDecl) []Func {
	var funcs []Func
	for _, spec := range d.Specs {
		vs, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}
		for i, value := range vs.Values {
			name := vs.Names[min(i, len(vs.Names)-1)].Name
			ast.Inspect(value, func(n ast.Node) bool {
				lit, isLit := n.(*ast.FuncLit)
				if isLit {
					funcs = append(funcs, goFunc(fset, name, lit.Body))
				}
				return !isLit
			})
		}
	}
	return funcs
}
