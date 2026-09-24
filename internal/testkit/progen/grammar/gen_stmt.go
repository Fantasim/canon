package grammar

import "github.com/fantasim/canonlang/internal/syntax"

// block is "{ statements }", one per line, in the context c adds to the enclosing one.
func (g *gen) block(c ctx) {
	g.w(openBrace)
	g.depth++
	g.with(g.ctx&^inHeader|c, func() {
		for n := g.r.Intn(maxStmts + 1); n > 0 && g.b.Enter(); n-- {
			g.nl()
			g.statement()
			g.b.Leave()
		}
	})
	g.depth--
	g.nl()
	g.w(closeBrace)
}

func (g *gen) statement() {
	for done := false; !done; {
		done = stmtKinds[g.r.Intn(len(stmtKinds))](g)
	}
	if g.r.OneIn(oneIn8) {
		g.w(space, linePrefix, g.note())
	}
}

func (g *gen) letStmt() bool {
	if g.r.OneIn(oneIn2) {
		g.tok(syntax.KwLet)
	} else {
		g.tok(syntax.KwVar)
	}
	g.w(space, g.lower())
	if g.r.OneIn(oneIn3) {
		g.w(colonSpace)
		g.typ()
	}
	g.w(assignOp)
	g.expr(0)
	return true
}

// ifStmt is "if h block [else (ifStmt | block)]".
func (g *gen) ifStmt() bool {
	g.tok(syntax.KwIf)
	g.w(space)
	g.header()
	g.w(space)
	g.block(0)
	if g.r.OneIn(oneIn2) {
		g.w(space)
		g.tok(syntax.KwElse)
		g.w(space)
		if g.r.OneIn(oneIn3) {
			return g.ifStmt()
		}
		g.block(0)
	}
	return true
}

func (g *gen) forStmt() bool {
	g.tok(syntax.KwFor)
	g.w(space, g.binder())
	if g.r.OneIn(oneIn3) {
		g.w(commaSpace, g.binder())
	}
	g.w(space)
	g.tok(syntax.KwIn)
	g.w(space)
	g.header()
	g.w(space)
	g.block(inLoop)
	return true
}

func (g *gen) whileStmt() bool {
	g.tok(syntax.KwWhile)
	g.w(space)
	g.header()
	g.w(space)
	g.block(inLoop)
	return true
}

// matchStmt is "match h { p => block | e }"; an arm's expression never starts with "{" (§6.5).
func (g *gen) matchStmt() bool {
	g.tok(syntax.KwMatch)
	g.w(space)
	g.header()
	g.w(space)
	g.body(func() {
		g.patterns()
		g.w(fatSpaced)
		if g.r.OneIn(oneIn2) {
			g.block(0)
			return
		}
		g.w(g.lower(), space)
		g.tok(pickOf(g.r, addOps))
		g.w(space)
		g.expr(levelMul)
	})
	return true
}

func (g *gen) jumpStmt() bool {
	if g.ctx&inLoop == 0 {
		return false
	}
	if g.r.OneIn(oneIn2) {
		g.tok(syntax.KwBreak)
	} else {
		g.tok(syntax.KwContinue)
	}
	return true
}

func (g *gen) returnStmt() bool {
	if g.ctx&inFn == 0 {
		return false
	}
	g.tok(syntax.KwReturn)
	if g.r.OneIn(oneIn4) {
		return true
	}
	g.w(space)
	g.expr(0)
	return true
}

// expectStmt is "expect e [fails|warns (s | name) | passes]" in a test block.
func (g *gen) expectStmt() bool {
	if g.ctx&inTest == 0 {
		return false
	}
	g.tok(syntax.KwExpect)
	g.w(space, g.lower(), space)
	g.tok(syntax.TokEq)
	g.w(space)
	g.closed(func() { g.expr(levelRange) })
	switch {
	case g.r.OneIn(oneIn4):
		g.w(passesWord)
	case g.r.OneIn(oneIn3):
		g.w(space, failsWord, space)
		g.constStr()
	case g.r.OneIn(oneIn2):
		g.w(space, warnsWord, space, g.lower())
	}
	return true
}

// simpleStmt is a call or an assignment to a name: no keyword or "{" a statement reads (§6.2).
func (g *gen) simpleStmt() bool {
	g.w(g.lower())
	if g.r.OneIn(oneIn2) {
		g.call()
		return true
	}
	if g.r.OneIn(oneIn3) {
		g.index()
	}
	g.w(space)
	g.tok(pickOf(g.r, assignOps))
	g.w(space)
	g.expr(0)
	return true
}

// reportStmt is fail(at, message) or warn(at, message), inside a check block only (E1105).
func (g *gen) reportStmt() bool {
	if g.ctx&inCheck == 0 {
		return false
	}
	if g.r.OneIn(oneIn2) {
		g.w(failCall)
	} else {
		g.w(warnCall)
	}
	g.with(0, func() {
		g.w(g.lower(), commaSpace)
		g.str()
	})
	g.w(closeParen)
	return true
}
