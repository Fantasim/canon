package gorules

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"

	"github.com/fantasim/canonlang/tools/audit/internal/finding"
)

// helper is a hand-written function the standard library already provides.
type helper struct{ pattern, std string }

// stdlib reports functions whose whole body is a contains, index or min/max helper, and
// loops anywhere that collect a map's keys into a slice, outside test code.
func (s *scan) stdlib() {
	for _, f := range s.files {
		if f.TestCode() {
			continue
		}
		for _, d := range f.AST.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			if h, ok := helperOf(fd); ok {
				s.emit(finding.Finding{
					Rule: ruleStdlib, File: f.Path, Line: s.line(fd.Name.Pos()), Detail: h.pattern,
					Message: fmt.Sprintf(msgStdlib, fd.Name.Name, h.std), Fix: fmt.Sprintf(fixUse, h.std),
				})
			}
		}
		ast.Inspect(f.AST, func(n ast.Node) bool {
			if list := stmtList(n); len(list) > 0 {
				s.keyLoops(f, list)
			}
			return true
		})
	}
}

func helperOf(fd *ast.FuncDecl) (helper, bool) {
	if h, ok := searchLoop(fd); ok {
		return h, true
	}
	return minMax(fd)
}

// searchLoop matches `for _, v := range xs { if v == x { return true|i } }; return false|-1`.
func searchLoop(fd *ast.FuncDecl) (helper, bool) {
	body := fd.Body.List
	if len(body) != pairLen {
		return helper{}, false
	}
	rs, ok := body[firstArg].(*ast.RangeStmt)
	last, ok2 := body[secondArg].(*ast.ReturnStmt)
	if !ok || !ok2 || len(last.Results) != 1 || len(rs.Body.List) != 1 {
		return helper{}, false
	}
	hit, ok := matchIf(rs)
	if !ok {
		return helper{}, false
	}
	kind := exprShape(rs.X)
	switch {
	case kind == shapeMap:
		return helper{}, false
	case isIdent(hit, identTrue) && isIdent(last.Results[firstArg], identFalse):
		return helper{patContains, pick(kind == shapeString, stdContainsRune, stdContains)}, true
	case rs.Key != nil && isIdent(hit, identName(rs.Key)) && minusOne(last.Results[firstArg]):
		return helper{patIndex, pick(kind == shapeString, stdIndexRune, stdIndex)}, true
	}
	return helper{}, false
}

// matchIf returns the result of `if elem == x { return r }`, the range body's only statement,
// where elem is the range value (or xs[i]) and x does not depend on the loop.
func matchIf(rs *ast.RangeStmt) (ast.Expr, bool) {
	ifs, ok := rs.Body.List[firstArg].(*ast.IfStmt)
	if !ok || ifs.Init != nil || ifs.Else != nil || len(ifs.Body.List) != 1 {
		return nil, false
	}
	ret, ok := ifs.Body.List[firstArg].(*ast.ReturnStmt)
	cond, ok2 := ifs.Cond.(*ast.BinaryExpr)
	if !ok || !ok2 || cond.Op != token.EQL || len(ret.Results) != 1 {
		return nil, false
	}
	loopVars := []string{identName(rs.Key), identName(rs.Value)}
	for _, pair := range [][pairLen]ast.Expr{{cond.X, cond.Y}, {cond.Y, cond.X}} {
		if isElem(rs, pair[firstArg]) && !mentions(pair[secondArg], loopVars) {
			return ret.Results[firstArg], true
		}
	}
	return nil, false
}

// isElem reports the range value itself, or xs[i] when only the index is bound.
func isElem(rs *ast.RangeStmt, e ast.Expr) bool {
	if v := identName(rs.Value); v != "" {
		return isIdent(e, v)
	}
	ix, ok := e.(*ast.IndexExpr)
	k := identName(rs.Key)
	return ok && k != "" && isIdent(ix.Index, k) && types.ExprString(ix.X) == types.ExprString(rs.X)
}

// minMax matches a two-parameter function returning the smaller or larger of them.
func minMax(fd *ast.FuncDecl) (helper, bool) {
	a, b, ok := twoParams(fd)
	if !ok {
		return helper{}, false
	}
	ifs, other, ok := ifReturn(fd.Body.List)
	if !ok {
		return helper{}, false
	}
	cond, ok := ifs.Cond.(*ast.BinaryExpr)
	then := ifs.Body.List[firstArg].(*ast.ReturnStmt).Results[firstArg]
	if !ok || !orderOp(cond.Op) || !samePair(cond.X, cond.Y, a, b) || !samePair(then, other, a, b) {
		return helper{}, false
	}
	less := cond.Op == token.LSS || cond.Op == token.LEQ
	if less == (identName(then) == identName(cond.X)) {
		return helper{patMin, stdMin}, true
	}
	return helper{patMax, stdMax}, true
}

// ifReturn matches `if c { return x }; return y` and `if c { return x } else { return y }`.
func ifReturn(body []ast.Stmt) (*ast.IfStmt, ast.Expr, bool) {
	if len(body) == 0 || len(body) > pairLen {
		return nil, nil, false
	}
	ifs, ok := body[firstArg].(*ast.IfStmt)
	if !ok || ifs.Init != nil || !singleReturn(ifs.Body.List) {
		return nil, nil, false
	}
	var rest []ast.Stmt
	switch {
	case len(body) == pairLen && ifs.Else == nil:
		rest = body[secondArg:]
	case len(body) == 1 && ifs.Else != nil:
		blk, ok := ifs.Else.(*ast.BlockStmt)
		if !ok {
			return nil, nil, false
		}
		rest = blk.List
	}
	if !singleReturn(rest) {
		return nil, nil, false
	}
	return ifs, rest[firstArg].(*ast.ReturnStmt).Results[firstArg], true
}

func singleReturn(list []ast.Stmt) bool {
	if len(list) != 1 {
		return false
	}
	r, ok := list[firstArg].(*ast.ReturnStmt)
	return ok && len(r.Results) == 1
}

func twoParams(fd *ast.FuncDecl) (string, string, bool) {
	var names []string
	for _, p := range fd.Type.Params.List {
		for _, n := range p.Names {
			names = append(names, n.Name)
		}
	}
	res := fd.Type.Results
	if len(names) != pairLen || res == nil || res.NumFields() != 1 {
		return "", "", false
	}
	return names[firstArg], names[secondArg], true
}

func orderOp(op token.Token) bool {
	return op == token.LSS || op == token.LEQ || op == token.GTR || op == token.GEQ
}

func samePair(x, y ast.Expr, a, b string) bool {
	p, q := identName(x), identName(y)
	return p != q && (p == a || p == b) && (q == a || q == b)
}

func minusOne(e ast.Expr) bool {
	u, ok := e.(*ast.UnaryExpr)
	if !ok || u.Op != token.SUB {
		return false
	}
	l, ok := u.X.(*ast.BasicLit)
	return ok && litIs(l.Value, l.Kind, 1)
}

func pick(cond bool, yes, no string) string {
	if cond {
		return yes
	}
	return no
}
