package grammar

import "github.com/fantasim/canonlang/internal/syntax"

func (g *gen) topDecl() {
	if g.r.OneIn(oneIn3) {
		g.doc()
	}
	declKinds[g.r.Intn(len(declKinds))](g)
}

// local writes "local " now and then, on the declarations that take it (GRAMMAR.md §5.3).
func (g *gen) local() {
	if g.r.OneIn(oneIn3) {
		g.tok(syntax.KwLocal)
		g.w(space)
	}
}

func (g *gen) constDecl() {
	g.prefixAnnotations(declAnns)
	g.local()
	g.tok(syntax.KwConst)
	g.w(space, g.upper(), assignOp)
	g.expr(0)
}

func (g *gen) letDecl() {
	g.prefixAnnotations(letAnns)
	g.local()
	g.tok(syntax.KwLet)
	g.w(space, g.lower())
	if g.r.OneIn(oneIn2) {
		g.w(colonSpace)
		g.typ()
	}
	g.w(assignOp)
	g.expr(0)
}

func (g *gen) typeDecl() {
	g.prefixAnnotations(declAnns)
	g.local()
	g.tok(syntax.KwType)
	g.w(space, g.upper())
	if g.r.OneIn(oneIn4) {
		g.typeParams()
	}
	g.w(assignOp)
	g.typ()
}

// typeParams are "(p: T [= e], …)", at least one.
func (g *gen) typeParams() {
	g.bracketed(openParen, closeParen, func() {
		g.list(1+g.r.Intn(maxArgs-1), g.param)
	})
}

func (g *gen) param() {
	g.w(g.lower(), colonSpace)
	g.typ()
	if g.r.OneIn(oneIn4) {
		g.w(assignOp)
		g.expr(levelCoal)
	}
}

func (g *gen) enumDecl() {
	g.local()
	g.tok(syntax.KwEnum)
	g.w(space, g.upper())
	if g.r.OneIn(oneIn4) {
		g.w(space, orderedWord)
	}
	g.suffix(enumAnns)
	g.w(space)
	g.body(g.enumMember)
}

func (g *gen) enumMember() {
	if g.r.OneIn(oneIn6) {
		g.tok(syntax.KwRetired)
		g.w(space)
	}
	g.w(g.pick(dataWords))
	switch {
	case g.r.OneIn(oneIn4):
		g.w(assignOp, quote, textPart, quote)
	case g.r.OneIn(oneIn3):
		g.w(assignOp, g.pick(ints))
	}
	g.suffix(memberAnns)
}

// body writes "{ … }" with up to maxItems items by f, one per line or on one line.
func (g *gen) body(f func()) { g.bodyList(f, false) }

// bodyList is body, a one-line list ending now and then with a separator when trailing.
func (g *gen) bodyList(f func(), trailing bool) {
	items := g.r.Intn(maxItems + 1)
	if items == 0 || !g.b.Enter() {
		g.w(emptyBraces)
		return
	}
	defer g.b.Leave()
	saved := g.lines
	defer func() { g.lines = saved }()
	if g.ctx&inInterp != 0 || g.r.OneIn(oneIn3) {
		g.lines = false
		g.w(braceSpaced)
		g.with(0, func() { g.list(items, f) })
		if trailing && g.r.OneIn(oneIn4) {
			g.w(",")
		}
		g.w(spacedBrace)
		return
	}
	g.w(openBrace)
	g.depth++
	g.with(0, func() {
		for range items {
			g.lines = true
			g.nl()
			f()
		}
	})
	g.depth--
	g.nl()
	g.w(closeBrace)
}

func (g *gen) recordDecl() {
	g.local()
	g.tok(syntax.KwRecord)
	g.w(space, g.upper())
	if g.r.OneIn(oneIn6) {
		g.typeParams()
	}
	g.suffix(recordAnns)
	g.w(space)
	g.body(g.recordItem)
}

func (g *gen) recordItem() {
	if g.r.OneIn(oneIn3) {
		g.memberFn()
		return
	}
	g.field()
}

// memberFn is a method or a check of a record or variant body, with its prefix annotations.
func (g *gen) memberFn() {
	g.annotations(methodAnns, "", space)
	if g.r.OneIn(oneIn2) {
		g.method()
		return
	}
	g.check()
}

func (g *gen) field() {
	if g.lines && g.r.OneIn(oneIn3) {
		g.doc()
	}
	g.w(g.lower(), colonSpace)
	if g.r.OneIn(oneIn8) {
		g.w(inputWord)
		g.typ()
		g.w(fromEnv, envName)
		return
	}
	g.typ()
	if g.r.OneIn(oneIn3) {
		g.w(assignOp)
		g.expr(0)
	}
	if g.r.OneIn(oneIn3) {
		g.w(space, g.pick(fieldJSON))
	}
	g.suffix(fieldAnns)
}

// method is a function of a record or variant body: self first, never local (E1133).
func (g *gen) method() {
	if g.r.OneIn(oneIn3) {
		g.tok(syntax.KwExport)
		g.w(space)
	}
	g.tok(syntax.KwFn)
	g.w(space, g.lower())
	g.bracketed(openParen, closeParen, func() {
		g.w(selfWord)
		for n := g.r.Intn(maxArgs); n > 0; n-- {
			g.w(commaSpace)
			g.param()
		}
	})
	g.w(arrowSpaced)
	g.resultType()
	g.w(space)
	g.block(inFn)
}

func (g *gen) variantDecl() {
	g.local()
	g.tok(syntax.KwVariant)
	g.w(space, g.upper())
	g.suffix(variantAnns)
	g.w(space)
	g.body(g.variantItem)
}

func (g *gen) variantItem() {
	if g.r.OneIn(oneIn6) {
		g.memberFn()
		return
	}
	if g.r.OneIn(oneIn8) {
		g.tok(syntax.KwRetired)
		g.w(space)
	}
	g.w(g.pick(dataWords))
	g.suffix(caseAnns)
	if g.r.OneIn(oneIn2) {
		g.w(space)
		g.body(g.field)
	}
}

func (g *gen) fnDecl() {
	g.prefixAnnotations(declAnns)
	switch {
	case g.r.OneIn(oneIn3):
		g.tok(syntax.KwLocal)
		g.w(space)
	case g.r.OneIn(oneIn2):
		g.tok(syntax.KwExport)
		g.w(space)
	}
	g.tok(syntax.KwFn)
	g.w(space, g.lower())
	g.bracketed(openParen, closeParen, func() { g.list(g.r.Intn(maxArgs), g.param) })
	g.w(arrowSpaced)
	g.resultType()
	g.w(space)
	g.block(inFn)
}

func (g *gen) entryDecl() {
	g.prefixAnnotations(entryAnns)
	if g.r.OneIn(oneIn4) {
		g.tok(syntax.KwRetired)
		g.w(space)
	}
	g.tok(syntax.KwEntry)
	g.w(space, g.lower(), ".")
	if g.r.OneIn(oneIn4) {
		g.w(g.pick(ints))
	} else {
		g.w(g.pick(dataWords))
	}
	g.w(space)
	g.braceLit()
}

func (g *gen) topCheck() {
	g.prefixAnnotations(anyAnns)
	g.check()
}

// check is a check or warn: a block, or a one-line condition with its message (§5.5).
func (g *gen) check() {
	if g.r.OneIn(oneIn2) {
		g.tok(syntax.KwCheck)
	} else {
		g.tok(syntax.KwWarn)
	}
	g.w(space)
	if g.r.OneIn(oneIn3) {
		g.block(inCheck)
		return
	}
	if g.r.OneIn(oneIn3) {
		g.w(g.lower(), colonSpace)
	}
	g.condition()
	if g.r.OneIn(oneIn3) {
		g.w(atWord, g.lower())
	}
	g.w(elseWord)
	g.str()
}

// condition is a comparison that starts with a name: no "{" a check line would read as its
// block (GRAM-11), no keyword.
func (g *gen) condition() {
	if g.r.OneIn(oneIn4) {
		g.tok(syntax.KwNot)
		g.w(space)
	}
	g.w(g.lower(), space)
	g.tok(pickOf(g.r, compareOps))
	g.w(space)
	g.closed(func() { g.expr(levelRange) })
}

func (g *gen) widgetDecl() {
	g.prefixAnnotations(anyAnns)
	g.tok(syntax.KwWidget)
	g.w(space, g.lower())
	g.bracketed(openParen, closeParen, func() {
		g.w(valueParam)
		g.typ()
		if g.r.OneIn(oneIn2) {
			g.w(siblings)
			g.typ()
		}
	})
	if g.r.OneIn(oneIn3) {
		g.w(defaultWord)
	}
}

func (g *gen) testDecl() {
	g.prefixAnnotations(anyAnns)
	g.tok(syntax.KwTest)
	g.w(space)
	g.constStr()
	g.w(space)
	g.block(inTest)
}

func (g *gen) emitDecl() {
	g.prefixAnnotations(anyAnns)
	g.tok(syntax.KwEmit)
	g.w(space, g.pick(emitTarget), space, braceSpaced, "out", colonSpace, quote, "out/", quote)
	if g.r.OneIn(oneIn2) {
		g.w(commaSpace, "mode", colonSpace, g.pick(emitModes))
	}
	if g.r.OneIn(oneIn3) {
		g.w(commaSpace, "values", colonSpace, openBrack, g.lower(), closeBrack)
	}
	g.w(spacedBrace)
}
