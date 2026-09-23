package gostyle

import "go/ast"

// maxNesting is the deepest if/for/range/switch/type-switch/select nesting inside body,
// plus, independently, inside every closure body found anywhere within it: a closure
// starts its own depth at 0 rather than adding to the depth around it.
func maxNesting(body *ast.BlockStmt) int {
	d := blockDepth(body, 0)
	ast.Inspect(body, func(n ast.Node) bool {
		if fl, ok := n.(*ast.FuncLit); ok {
			d = max(d, blockDepth(fl.Body, 0))
		}
		return true
	})
	return d
}

func blockDepth(b *ast.BlockStmt, depth int) int {
	d := depth
	for _, st := range b.List {
		d = max(d, stmtDepth(st, depth))
	}
	return d
}

// stmtDepth is the deepest nesting reached by st, given it sits at depth already. Only
// if/for/range/switch/type-switch/select add a level; a bare block or label does not.
func stmtDepth(s ast.Stmt, depth int) int {
	switch st := s.(type) {
	case *ast.IfStmt:
		return ifDepth(st, depth)
	case *ast.ForStmt:
		return blockDepth(st.Body, depth+1)
	case *ast.RangeStmt:
		return blockDepth(st.Body, depth+1)
	case *ast.SwitchStmt:
		return caseDepth(st.Body.List, depth+1)
	case *ast.TypeSwitchStmt:
		return caseDepth(st.Body.List, depth+1)
	case *ast.SelectStmt:
		return caseDepth(st.Body.List, depth+1)
	case *ast.BlockStmt:
		return blockDepth(st, depth)
	case *ast.LabeledStmt:
		return stmtDepth(st.Stmt, depth)
	}
	return depth
}

// ifDepth handles the "else if" chain at the same depth as the if it continues, while a
// plain "else { }" block nests one level, same as the if's own body.
func ifDepth(st *ast.IfStmt, depth int) int {
	d := blockDepth(st.Body, depth+1)
	switch e := st.Else.(type) {
	case *ast.IfStmt:
		d = max(d, ifDepth(e, depth))
	case *ast.BlockStmt:
		d = max(d, blockDepth(e, depth+1))
	}
	return d
}

func caseDepth(clauses []ast.Stmt, depth int) int {
	d := depth
	for _, c := range clauses {
		for _, st := range clauseBody(c) {
			d = max(d, stmtDepth(st, depth))
		}
	}
	return d
}

func clauseBody(c ast.Stmt) []ast.Stmt {
	switch cc := c.(type) {
	case *ast.CaseClause:
		return cc.Body
	case *ast.CommClause:
		return cc.Body
	}
	return nil
}
