package grammar

import "github.com/fantasim/canonlang/internal/syntax"

// expr is an expression of level min or tighter; a leaf once the budget is spent.
func (g *gen) expr(min int) {
	if !g.b.Enter() {
		g.leaf()
		return
	}
	defer g.b.Leave()
	levels[min+g.r.Intn(levelCount-min)](g)
}

func (g *gen) binary(left int, ops []syntax.TokenKind, right int) {
	g.expr(left)
	g.sp()
	g.tok(pickOf(g.r, ops))
	g.sp()
	g.expr(right)
}

// lambda is "x => e" or "(a, b) => e"; its body extends as far as possible (§6.3).
func (g *gen) lambda() {
	if g.r.OneIn(oneIn2) {
		g.w(g.binder())
	} else {
		g.bracketed(openParen, closeParen, func() {
			g.list(g.r.Intn(maxArgs), func() { g.w(g.binder()) })
		})
	}
	g.w(fatSpaced)
	g.expr(0)
}

func (g *gen) binder() string {
	if g.r.OneIn(oneIn6) {
		return anyType
	}
	return g.lower()
}

func (g *gen) coalesce() {
	g.binary(levelOr, []syntax.TokenKind{syntax.TokCoalesce}, levelCoal)
}

func (g *gen) orExpr() { g.binary(levelOr, []syntax.TokenKind{syntax.KwOr}, levelAnd) }

func (g *gen) andExpr() { g.binary(levelAnd, []syntax.TokenKind{syntax.KwAnd}, levelNot) }

func (g *gen) notExpr() {
	g.tok(syntax.KwNot)
	g.w(space)
	g.expr(levelNot)
}

// compare is "a op b" or "a is W", operands of range level: a comparison never chains.
func (g *gen) compare() {
	if g.r.OneIn(oneIn4) {
		g.expr(levelRange)
		g.w(space)
		g.tok(syntax.KwIs)
		g.w(space, g.pick(dataWords))
		return
	}
	g.binary(levelRange, compareOps, levelRange)
}

// rangeExpr is "a..b", "..b", or "a.." where no "{" or word follows (§5.11).
func (g *gen) rangeExpr() {
	switch {
	case g.r.OneIn(oneIn3):
		g.tok(pickOf(g.r, rangeOps))
		g.expr(levelAdd)
	case g.r.OneIn(oneIn3) && g.ctx&(inHeader|inClosed) == 0:
		g.expr(levelAdd)
		g.tok(syntax.TokRange)
	default:
		g.expr(levelAdd)
		g.tok(pickOf(g.r, rangeOps))
		g.expr(levelAdd)
	}
}

func (g *gen) additive() { g.binary(levelAdd, addOps, levelMul) }

func (g *gen) multiplicative() { g.binary(levelMul, mulOps, levelUnary) }

func (g *gen) unary() {
	g.tok(syntax.TokMinus)
	g.expr(levelUnary)
}

// postfix is a primary and up to three postfix steps; a number literal takes none, since "1."
// would lex as part of it.
func (g *gen) postfix() {
	if g.primary() {
		return
	}
	for n := g.r.Intn(oneIn4); n > 0 && g.b.Spend(); n-- {
		postfixOps[g.r.Intn(len(postfixOps))](g)
	}
}

func (g *gen) member() {
	g.tok(syntax.TokDot)
	g.w(g.pick(wordPool))
}

func (g *gen) optMember() {
	g.tok(syntax.TokOptDot)
	g.w(g.pick(wordPool))
}

func (g *gen) index() { g.bracketed(openBrack, closeBrack, func() { g.expr(0) }) }

func (g *gen) force() { g.tok(syntax.TokBang) }

// call is "(args)": positional arguments, then one named argument (E1121).
func (g *gen) call() {
	g.bracketed(openParen, closeParen, func() {
		n := g.r.Intn(maxArgs)
		g.list(n, g.argument)
		if g.r.OneIn(oneIn3) {
			if n > 0 {
				g.w(commaSpace)
			}
			g.w(lowerNames[0], colonSpace)
			g.expr(0)
		}
	})
}

func (g *gen) argument() {
	if g.r.OneIn(oneIn6) {
		g.shorthand()
		return
	}
	g.expr(0)
}

// shorthand is ".w" and its postfix steps, an implicit lambda (§5.11).
func (g *gen) shorthand() {
	g.member()
	if g.r.OneIn(oneIn3) {
		g.call()
	}
}

// leaf is an expression that nests nothing.
func (g *gen) leaf() {
	switch g.r.Intn(oneIn4) {
	case 0:
		g.w(g.pick(ints))
	case 1:
		g.str()
	default:
		g.w(g.lower())
	}
}
