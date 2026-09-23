package gorules

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"slices"

	"github.com/fantasim/canonlang/tools/audit/internal/finding"
)

// keyLoops reports `for k := range m { keys = append(keys, k) }` in list, when m is known to
// be a map or keys was sized by len(m) just before.
func (s *scan) keyLoops(f *src, list []ast.Stmt) {
	for i, st := range list {
		rs, ok := st.(*ast.RangeStmt)
		if !ok {
			continue
		}
		dst, ok := keyAppend(rs)
		if !ok || exprShape(rs.X) != shapeMap && (i == 0 || !sizedBy(list[i-1], dst, rs.X)) {
			continue
		}
		m := types.ExprString(rs.X)
		std := fmt.Sprintf(stdKeys, m)
		if i+1 < len(list) && f.sorts(list[i+1], dst) {
			std = fmt.Sprintf(stdSortedKeys, m)
		}
		s.emit(finding.Finding{
			Rule: ruleStdlib, File: f.Path, Line: s.line(rs.Pos()), Detail: patKeys + " " + m,
			Message: fmt.Sprintf(msgKeys, m, std), Fix: fmt.Sprintf(fixUse, std),
		})
	}
}

// keyAppend returns dst of a range loop whose only statement is dst = append(dst, key).
func keyAppend(rs *ast.RangeStmt) (string, bool) {
	k := identName(rs.Key)
	v := identName(rs.Value)
	if k == "" || k == underscore || v != "" && v != underscore || len(rs.Body.List) != 1 {
		return "", false
	}
	as, ok := rs.Body.List[firstArg].(*ast.AssignStmt)
	if !ok || as.Tok != token.ASSIGN || len(as.Lhs) != 1 || len(as.Rhs) != 1 {
		return "", false
	}
	dst := identName(as.Lhs[firstArg])
	c, ok := as.Rhs[firstArg].(*ast.CallExpr)
	if !ok || dst == "" || !isBuiltin(c.Fun, fnAppend) || c.Ellipsis.IsValid() || len(c.Args) != pairLen {
		return "", false
	}
	return dst, isIdent(c.Args[firstArg], dst) && isIdent(c.Args[secondArg], k)
}

// sizedBy reports `dst := make([]T, 0, len(x))` (or its var form).
func sizedBy(prev ast.Stmt, dst string, x ast.Expr) bool {
	var lhs, rhs []ast.Expr
	switch p := prev.(type) {
	case *ast.AssignStmt:
		lhs, rhs = p.Lhs, p.Rhs
	case *ast.DeclStmt:
		if g, ok := p.Decl.(*ast.GenDecl); ok && len(g.Specs) == 1 {
			if vs, ok := g.Specs[firstArg].(*ast.ValueSpec); ok && len(vs.Names) == 1 {
				lhs, rhs = []ast.Expr{vs.Names[firstArg]}, vs.Values
			}
		}
	}
	if len(lhs) != 1 || len(rhs) != 1 || !isIdent(lhs[firstArg], dst) {
		return false
	}
	c, ok := rhs[firstArg].(*ast.CallExpr)
	if !ok || !isBuiltin(c.Fun, fnMake) || len(c.Args) != thirdArg+1 {
		return false
	}
	n, ok := c.Args[thirdArg].(*ast.CallExpr)
	return ok && isBuiltin(n.Fun, fnLen) && len(n.Args) == 1 && types.ExprString(n.Args[firstArg]) == types.ExprString(x)
}

// sorts reports a natural-order sort of dst: sort.Strings/Ints/Float64s, slices.Sort, or
// sort.Slice(dst, func(i, j int) bool { return dst[i] < dst[j] }).
func (f *src) sorts(st ast.Stmt, dst string) bool {
	es, ok := st.(*ast.ExprStmt)
	if !ok {
		return false
	}
	c, ok := es.X.(*ast.CallExpr)
	if !ok || len(c.Args) == 0 || !isIdent(c.Args[firstArg], dst) {
		return false
	}
	pkg, name := f.callee(c)
	switch {
	case len(c.Args) == 1:
		return slices.Contains(naturalSorts[pkg], name)
	case len(c.Args) == pairLen && f.calls(c, pkgSort, sortSlice, sortSliceStable):
		return ascending(c.Args[secondArg], dst)
	}
	return false
}

// ascending matches func(i, j int) bool { return dst[i] < dst[j] }.
func ascending(fn ast.Expr, dst string) bool {
	fl, ok := fn.(*ast.FuncLit)
	if !ok || len(fl.Type.Params.List) != 1 || len(fl.Type.Params.List[firstArg].Names) != pairLen || len(fl.Body.List) != 1 {
		return false
	}
	ret, ok := fl.Body.List[firstArg].(*ast.ReturnStmt)
	if !ok || len(ret.Results) != 1 {
		return false
	}
	cmp, ok := ret.Results[firstArg].(*ast.BinaryExpr)
	names := fl.Type.Params.List[firstArg].Names
	return ok && cmp.Op == token.LSS && indexes(cmp.X, dst, names[firstArg].Name) && indexes(cmp.Y, dst, names[secondArg].Name)
}

// indexes matches dst[i].
func indexes(e ast.Expr, dst, i string) bool {
	ix, ok := e.(*ast.IndexExpr)
	return ok && isIdent(ix.X, dst) && isIdent(ix.Index, i)
}

func stmtList(n ast.Node) []ast.Stmt {
	switch n := n.(type) {
	case *ast.BlockStmt:
		return n.List
	case *ast.CaseClause:
		return n.Body
	case *ast.CommClause:
		return n.Body
	}
	return nil
}

// exprShape tells a map, slice or string from its declaration in the same file, when the
// parser resolved it: a parameter's type, a var's type or value, an assignment's value.
func exprShape(x ast.Expr) shape {
	id, ok := x.(*ast.Ident)
	if !ok || id.Obj == nil {
		return shapeUnknown
	}
	switch d := id.Obj.Decl.(type) {
	case *ast.Field:
		return typeShape(d.Type)
	case *ast.ValueSpec:
		if d.Type != nil {
			return typeShape(d.Type)
		}
		return valueAt(d.Values, slices.IndexFunc(d.Names, func(n *ast.Ident) bool { return n.Name == id.Name }))
	case *ast.AssignStmt:
		return valueAt(d.Rhs, slices.IndexFunc(d.Lhs, func(e ast.Expr) bool { return isIdent(e, id.Name) }))
	}
	return shapeUnknown
}

func valueAt(values []ast.Expr, i int) shape {
	if i < 0 || i >= len(values) {
		return shapeUnknown
	}
	switch v := values[i].(type) {
	case *ast.CompositeLit:
		return typeShape(v.Type)
	case *ast.CallExpr:
		if isBuiltin(v.Fun, fnMake) && len(v.Args) > 0 {
			return typeShape(v.Args[firstArg])
		}
	case *ast.BasicLit:
		if v.Kind == token.STRING {
			return shapeString
		}
	}
	return shapeUnknown
}

func typeShape(t ast.Expr) shape {
	switch t := t.(type) {
	case *ast.ArrayType, *ast.Ellipsis:
		return shapeSlice
	case *ast.MapType:
		return shapeMap
	case *ast.Ident:
		if t.Name == identString {
			return shapeString
		}
	}
	return shapeUnknown
}

func identName(e ast.Expr) string {
	if id, ok := e.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

func isIdent(e ast.Expr, name string) bool { return name != "" && identName(e) == name }

// isBuiltin reports an unshadowed reference to a predeclared function.
func isBuiltin(e ast.Expr, name string) bool {
	id, ok := e.(*ast.Ident)
	return ok && id.Obj == nil && id.Name == name
}

// mentions reports whether e refers to any of names.
func mentions(e ast.Expr, names []string) bool {
	found := false
	ast.Inspect(e, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && id.Name != "" && slices.Contains(names, id.Name) {
			found = true
		}
		return !found
	})
	return found
}
