package grammar

import "github.com/fantasim/canonlang/internal/syntax"

// typ is a type: a union of optional types, each optionally refined by where (§5.9).
func (g *gen) typ() {
	g.optType()
	if g.r.OneIn(oneIn8) {
		g.w(space)
		g.tok(syntax.TokPipe)
		g.w(space)
		g.optType()
	}
}

func (g *gen) optType() {
	fn := g.primType()
	if !fn && g.r.OneIn(oneIn4) {
		g.tok(syntax.TokQuestion)
	}
	if !fn && g.ctx&inResult == 0 && g.r.OneIn(oneIn8) {
		g.w(space)
		g.tok(syntax.KwWhere)
		g.w(space)
		g.with(inClosed, func() { g.expr(levelCompare) })
	}
}

// primType writes one primType and tells whether it was a function type, whose own "?" and
// result a following "?" or where would be read as part of.
func (g *gen) primType() bool {
	if !g.b.Enter() || g.r.OneIn(oneIn2) {
		g.namedType()
		return false
	}
	defer g.b.Leave()
	g.fnType = false
	primTypes[g.r.Intn(len(primTypes))](g)
	return g.fnType
}

func (g *gen) namedType() {
	if g.r.OneIn(oneIn6) {
		g.w(anyType)
		return
	}
	g.w(g.qualified())
	if g.r.OneIn(oneIn3) {
		g.typeArgs(true)
	}
}

// typeArgs are one regex (a named type's only, §2.7) or up to two expressions (§5.9).
func (g *gen) typeArgs(regex bool) {
	g.bracketed(openParen, closeParen, func() {
		if regex && g.r.OneIn(oneIn3) {
			g.regex()
			return
		}
		g.list(1+g.r.Intn(maxArgs-1), g.refinement)
	})
}

// refinement is a range with one or both bounds, or an expression.
func (g *gen) refinement() {
	switch g.r.Intn(oneIn3) {
	case 0:
		g.w(g.pick(ints))
		g.tok(syntax.TokRange)
	case 1:
		g.tok(pickOf(g.r, rangeOps))
		g.w(g.pick(ints))
	default:
		g.expr(levelCoal)
	}
}

func (g *gen) listType() {
	g.bracketed(openBrack, closeBrack, g.typ)
	if g.r.OneIn(oneIn4) {
		g.typeArgs(false)
	}
	if g.r.OneIn(oneIn4) {
		g.w(space, keyedWord, space, byWord, space, g.lower())
	}
}

func (g *gen) mapType() {
	g.bracketed(openBrace, closeBrace, func() {
		g.typ()
		g.w(colonSpace)
		g.typ()
	})
}

func (g *gen) depMapType() {
	g.bracketed(openBrace, closeBrace, func() {
		g.w(g.lower(), space)
		g.tok(syntax.KwIn)
		g.w(space, g.lower(), colonSpace)
		g.typ()
	})
}

func (g *gen) tableType() {
	if g.r.OneIn(oneIn2) {
		g.tok(syntax.KwStable)
		g.w(space)
	}
	g.tok(syntax.KwTable)
	g.w(space, g.qualified())
}

func (g *gen) refType() {
	g.tok(syntax.KwRef)
	g.w(space, g.qualified())
}

// fnType is "fn(T, …) -> R", R a primType possibly optional (§5.9, GRM-15).
func (g *gen) funcType() {
	g.tok(syntax.KwFn)
	g.bracketed(openParen, closeParen, func() { g.list(g.r.Intn(maxArgs), g.typ) })
	g.w(arrowSpaced)
	g.namedType()
	if g.r.OneIn(oneIn3) {
		g.tok(syntax.TokQuestion)
	}
	g.fnType = true
}

func (g *gen) assetType() {
	g.tok(syntax.KwAsset)
	g.bracketed(openParen, closeParen, func() {
		g.constStr()
		if g.r.OneIn(oneIn2) {
			g.w(commaSpace, extWord, colonSpace, openBrack, g.lower(), commaSpace, quote, g.lower(), quote, closeBrack)
		}
	})
}

// matchType is "match h { p => T, … }", its header in header mode (§6.1).
func (g *gen) matchType() {
	g.tok(syntax.KwMatch)
	g.w(space)
	g.header()
	g.w(space)
	g.body(func() {
		g.patterns()
		g.w(fatSpaced)
		g.typ()
	})
}

func (g *gen) literalType() { g.constStr() }

func (g *gen) parenType() { g.bracketed(openParen, closeParen, g.typ) }

// resultType is a function's result type: no where, whose expression would read the body's "{"
// as a typed literal.
func (g *gen) resultType() {
	saved := g.ctx
	g.ctx |= inResult
	g.typ()
	g.ctx = saved
}
