package edit

import (
	"github.com/fantasim/canonlang/internal/syntax"
)

// keyLiteral reports a key written as a string or integer literal that reads the target's entry
// from a receiver reaching its collection statically, `xs["k"]` or `xs.hasKey("k")` (also `get` and
// `find`): a ref of that entry, as the name it may replace is (API.md R7; DECISIONS 366, 477).
func (sc *nameScan) keyLiteral(n syntax.Node, at scanAt) {
	recv, k := sc.literalRead(n)
	if _, isCall := n.(*syntax.CallExpr); recv == nil || sc.tg.ident == nil || isCall && sc.atHome(recv) {
		return // a home field's key test is keyTest's; an index is this one's
	}
	if match, literal := sc.targetKey(k); !match || !literal {
		return
	}
	if _, named := sc.names(k); named {
		return
	}
	p, ok := sc.s.StaticPath(recv)
	if !ok {
		return
	}
	res, err := Resolve(sc.s, sc.s.unalias(p))
	if err == nil && sc.tg.isEntry(keyedIdent(res.Target, sc.tg.ident.Key)) {
		sc.listKey(k, at)
	}
}

// literalRead is the receiver and the literal key of n when n reads a collection by a key written
// as a string or integer literal: an index, or a never-failing key test with one argument.
func (sc *nameScan) literalRead(n syntax.Node) (syntax.Expr, syntax.Expr) {
	var recv, k syntax.Expr
	switch x := n.(type) {
	case *syntax.IndexExpr:
		recv, k = x.X, syntax.Unparen(x.Index)
	case *syntax.CallExpr:
		sel, isSel := syntax.Unparen(x.Fun).(*syntax.SelectorExpr)
		if !isSel || len(x.Args) != 1 || x.Args[0].Value == nil || !neverFails(sc.s.info.Types[x]) {
			return nil, nil
		}
		recv, k = sel.X, syntax.Unparen(x.Args[0].Value)
	default:
		return nil, nil
	}
	switch k.(type) {
	case *syntax.StringLit, *syntax.RawStringLit, *syntax.IntLit:
		return recv, k
	}
	return nil, nil
}
