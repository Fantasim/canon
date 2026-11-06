package edit

import (
	"github.com/fantasim/canonlang/internal/syntax"
)

// keyLiteral reports a key written as a string or integer literal, or a member, that reads the
// target's entry from a receiver reaching its collection statically, `xs["k"]` or `xs.hasKey("k")`
// (also `get` and `find`): a ref of that entry, as the name it may replace is (API.md R7; DECISIONS 366, 477, 490).
func (sc *nameScan) keyLiteral(n syntax.Node, at scanAt) {
	recv, k := sc.literalRead(n)
	_, isCall := n.(*syntax.CallExpr)
	if recv == nil || sc.tg.ident == nil || isCall && sc.atHome(recv) || !isCall && sc.elemKeyed(recv, k) {
		return // a home field's key test is keyTest's, a member index on a ref-keyed list elemKey's
	}
	tok, match, literal := sc.keyOf(k)
	if !match || !literal {
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
		sc.listKey(tok, at)
		return
	}
	sc.keyReadBase(recv, tok, at)
}

// keyReadBase judges a key read of the target's key the layered evaluation does not reach it by:
// listed when the base one does, and either way, when the base cannot keep it, a shared path that
// refuses the rename `computed` (API.md R7, E12; DECISIONS 483, 489, 503).
func (sc *nameScan) keyReadBase(recv syntax.Expr, tok syntax.Node, at scanAt) {
	read, p := sc.readBase(recv)
	if read.holds {
		sc.listKey(tok, at)
	}
	if read.stuck() {
		sc.shared = append(sc.shared, sc.tg.readPath(p))
	}
}

// elemKeyed reports k a member keying recv, a list keyed by a ref, which elemKey judges.
func (sc *nameScan) elemKeyed(recv, k syntax.Expr) bool {
	_, _, member := sc.s.memberToken(k)
	return member && refKeyed(sc.s.info.Types[recv])
}

// literalRead is the receiver and the literal key of n when n reads a collection by a key written
// as a string or integer literal, or a member: an index, or a never-failing key test with one argument.
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
	if _, _, member := sc.s.memberToken(k); member {
		return recv, k // a member key, bare or qualified (DECISIONS 490)
	}
	return nil, nil
}
