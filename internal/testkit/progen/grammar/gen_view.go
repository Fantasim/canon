package grammar

import "github.com/fantasim/canonlang/internal/syntax"

func (g *gen) viewDecl() {
	g.prefixAnnotations(anyAnns)
	g.tok(syntax.KwView)
	g.w(space, g.upper())
	if g.r.OneIn(oneIn4) {
		g.w(".", g.pick(dataWords))
	}
	g.w(space)
	g.body(func() { viewItems[g.r.Intn(len(viewItems))](g) })
}

func (g *gen) viewText() {
	g.w(g.pick(viewTexts), space)
	g.str()
}

func (g *gen) viewMenu() { g.w("menu ", g.pick(dataWords), " icon ", g.pick(dataWords)) }

func (g *gen) viewPreview() {
	g.w("preview ")
	g.w(g.lower())
	if g.r.OneIn(oneIn2) {
		g.member()
	}
}

func (g *gen) viewSearch() {
	g.w("search ")
	g.bracketed(braceSpaced, spacedBrace, func() {
		g.list(1+g.r.Intn(maxArgs), func() {
			g.w(g.lower())
			g.member()
		})
	})
}

func (g *gen) viewFilters() {
	g.w("filters ")
	g.bracketed(braceSpaced, spacedBrace, func() {
		g.list(1+g.r.Intn(maxArgs), func() {
			g.w(g.lower())
			if g.r.OneIn(oneIn2) {
				g.w(multiWord)
			}
		})
	})
}

func (g *gen) viewColumns() {
	g.w("columns ")
	g.bracketed(braceSpaced, spacedBrace, func() {
		g.list(1+g.r.Intn(maxArgs), func() {
			g.w(g.lower())
			if g.r.OneIn(oneIn2) {
				g.w(space, g.pick(ints))
			}
		})
	})
}

// viewGroup is "group id "label" ["intro"] [advanced] [when h] { members }".
func (g *gen) viewGroup() {
	g.w("group ", g.pick(dataWords), space)
	g.constStr()
	if g.r.OneIn(oneIn3) {
		g.w(space)
		g.constStr()
	}
	if g.r.OneIn(oneIn3) {
		g.w(advanced)
	}
	if g.r.OneIn(oneIn3) {
		g.w(whenWord)
		g.header()
	}
	g.w(space)
	g.body(func() {
		if g.r.OneIn(oneIn3) {
			g.viewShow()
			return
		}
		g.viewField()
	})
}

func (g *gen) viewShow() {
	g.w("show ")
	if g.r.OneIn(oneIn2) {
		g.w(g.lower(), space)
	}
	g.constStr()
	g.w(space)
	g.str()
}

// viewField is "[field] name ["label"] [{ props }]".
func (g *gen) viewField() {
	if g.r.OneIn(oneIn2) {
		g.w(fieldWord, space, g.pick(viewTexts))
	} else {
		g.w(g.lower())
	}
	if g.r.OneIn(oneIn2) {
		g.w(space)
		g.constStr()
	}
	if g.r.OneIn(oneIn3) {
		g.w(space)
		g.bracketed(braceSpaced, spacedBrace, func() {
			g.w(g.pick(dataWords), colonSpace)
			g.expr(levelCoal)
		})
	}
}
