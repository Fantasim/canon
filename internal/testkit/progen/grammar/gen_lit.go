package grammar

import (
	"strings"

	"github.com/fantasim/canonlang/internal/syntax"
)

func (g *gen) primary() bool { return primaries[g.r.Intn(len(primaries))](g) }

func (g *gen) number() bool {
	switch {
	case g.r.OneIn(oneIn3):
		g.w(g.pick(floats))
	case g.r.OneIn(oneIn2):
		g.w(g.pick(durations))
	default:
		g.w(g.pick(ints))
	}
	return true
}

func (g *gen) stringPrimary() bool {
	g.str()
	return false
}

// namePrimary is an identifier, a nameable reserved word (§4.3 c) or self.
func (g *gen) namePrimary() bool {
	switch {
	case g.r.OneIn(oneIn6):
		g.w(g.pick(nameable))
	case g.r.OneIn(oneIn6):
		g.w(selfWord)
	case g.r.OneIn(oneIn4):
		g.w(g.upper())
	default:
		g.w(g.lower())
	}
	return false
}

func (g *gen) parenPrimary() bool {
	g.bracketed(openParen, closeParen, func() { g.expr(0) })
	return false
}

func (g *gen) constPrimary() bool {
	g.tok(pickOf(g.r, constWords))
	return false
}

// listPrimary is "[a, b]" or a comprehension "[e for x in xs if c]".
func (g *gen) listPrimary() bool {
	g.bracketed(openBrack, closeBrack, func() {
		if g.r.OneIn(oneIn4) {
			g.expr(0)
			g.clauses()
			return
		}
		g.list(g.r.Intn(maxItems), func() { g.expr(0) })
	})
	return false
}

// clauses are a comprehension's for, if and let clauses, a for first; they are no headers.
func (g *gen) clauses() {
	g.w(space)
	g.tok(syntax.KwFor)
	g.w(space, g.binder(), space)
	g.tok(syntax.KwIn)
	g.w(space)
	g.closed(func() { g.expr(levelRange) })
	if g.r.OneIn(oneIn3) {
		g.w(space)
		g.tok(syntax.KwIf)
		g.w(space)
		g.closed(func() { g.expr(levelOr) })
	}
	if g.r.OneIn(oneIn4) {
		g.w(space)
		g.tok(syntax.KwLet)
		g.w(space, g.lower(), assignOp)
		g.closed(func() { g.expr(levelOr) })
	}
}

// guarded writes f, parenthesized in a header where its "{" would end the header (E1129).
func (g *gen) guarded(f func()) bool {
	if g.ctx&inHeader != 0 {
		g.bracketed(openParen, closeParen, f)
		return false
	}
	f()
	return false
}

func (g *gen) bracePrimary() bool { return g.guarded(g.braceLit) }

func (g *gen) typedPrimary() bool {
	return g.guarded(func() {
		g.w(g.qualified(), space)
		g.braceLit()
	})
}

func (g *gen) ifPrimary() bool { return g.guarded(g.ifExpr) }

func (g *gen) matchPrimary() bool { return g.guarded(g.matchExpr) }

func (g *gen) loadPrimary() bool {
	if g.r.OneIn(oneIn2) {
		g.tok(syntax.KwLoad)
		g.bracketed(openParen, closeParen, func() { g.w(quote, "data.json", quote) })
		return false
	}
	g.w(loadDir)
	g.bracketed(openParen, closeParen, func() { g.w(quote, "data/*.json", quote) })
	return false
}

// braceLit is a brace literal: items (spread, entry, named, keyed) or one comprehension item.
func (g *gen) braceLit() {
	if g.r.OneIn(oneIn6) && g.b.Spend() {
		g.bracketed(braceSpaced, spacedBrace, func() {
			g.w(g.pick(dataWords), colonSpace)
			g.expr(levelCoal)
			g.clauses()
		})
		return
	}
	g.body(g.braceItem)
}

func (g *gen) braceItem() {
	switch {
	case g.r.OneIn(oneIn6):
		g.tok(syntax.TokEllipsis)
		g.expr(levelPostfix)
	case g.r.OneIn(oneIn6):
		g.w(g.pick(dataWords))
		g.suffix(entryAnns)
		g.w(space)
		g.braceLit()
	case g.r.OneIn(oneIn4):
		g.w(quote, textPart, quote, colonSpace)
		g.expr(0)
	default:
		g.w(g.pick(dataWords), colonSpace)
		g.expr(0)
	}
}

// ifExpr is "if h { e } else if h { e } else { e }", its headers in header mode (§6.1).
func (g *gen) ifExpr() {
	g.tok(syntax.KwIf)
	g.w(space)
	g.header()
	g.w(space)
	g.bracketed(braceSpaced, spacedBrace, func() { g.expr(0) })
	if g.r.OneIn(oneIn3) {
		g.w(space)
		g.tok(syntax.KwElse)
		g.w(space)
		g.ifExpr()
		return
	}
	g.w(space)
	g.tok(syntax.KwElse)
	g.w(space)
	g.bracketed(braceSpaced, spacedBrace, func() { g.expr(0) })
}

// matchExpr is "match h { p => e, … }": after "=>" a "{" starts a brace literal (§6.5).
func (g *gen) matchExpr() {
	g.tok(syntax.KwMatch)
	g.w(space)
	g.header()
	g.w(space)
	g.body(func() {
		g.patterns()
		g.w(fatSpaced)
		g.expr(levelCoal)
	})
}

// header is an expression in header mode: no "{" at its depth 0 (§6.1).
func (g *gen) header() {
	saved := g.ctx
	g.ctx |= inHeader
	g.expr(levelCoal)
	g.ctx = saved
}

// patterns are one to three patterns: "_", "none", or a word with an optional binder.
func (g *gen) patterns() {
	g.list(1+g.r.Intn(maxArgs), func() {
		switch {
		case g.r.OneIn(oneIn6):
			g.w(anyType)
		case g.r.OneIn(oneIn6):
			g.tok(syntax.KwNone)
		case g.r.OneIn(oneIn4):
			g.w(g.pick(dataWords), openParen, g.binder(), closeParen)
		default:
			g.w(g.pick(dataWords))
		}
	})
}

// regex is a regular expression literal; only where §2.7 accepts one.
func (g *gen) regex() { g.w(regexSlash, g.pick(regexes), regexSlash) }

// str is a string literal of any form: plain, raw, multiline or raw multiline (§2.6).
func (g *gen) str() {
	switch {
	case g.ctx&inInterp == 0 && g.r.OneIn(oneIn8):
		g.multiline(g.r.OneIn(oneIn2))
	case g.r.OneIn(oneIn6):
		g.w(rawPrefix, quote, g.runs(rawRunes), quote)
	default:
		g.plain()
	}
}

// constStr is a string with no interpolation, where §2.6 asks for a constant one.
func (g *gen) constStr() {
	if g.r.OneIn(oneIn4) {
		g.w(rawPrefix, quote, g.runs(rawRunes), quote)
		return
	}
	g.w(quote, g.runs(textRunes), quote)
}

func (g *gen) runs(pool []string) string {
	var b strings.Builder
	for n := g.r.Intn(maxItems); n > 0; n-- {
		b.WriteString(g.pick(pool))
	}
	return b.String()
}

// plain is a string with text, escapes and interpolations "{e}" or "{e:spec}".
func (g *gen) plain() {
	g.w(quote)
	g.parts()
	g.w(quote)
}

func (g *gen) parts() {
	for n := g.r.Intn(maxParts + 1); n > 0; n-- {
		switch g.r.Intn(oneIn4) {
		case 0:
			g.w(g.pick(escapes))
		case 1:
			if !g.b.Spend() {
				continue
			}
			g.interpolation()
		default:
			g.w(g.runs(textRunes))
		}
	}
}

// interpolation is "{e}" or "{e:spec}" on one line; a header stays one inside it (§6.1 is mute).
func (g *gen) interpolation() {
	saved := g.ctx
	g.ctx |= inInterp
	g.w(interpOpen)
	start := g.out.Len()
	g.expr(levelCoal)
	g.unbrace(start)
	if g.r.OneIn(oneIn3) {
		g.w(specColon, g.pick(specs))
	} else if strings.HasSuffix(g.out.String(), closeBrace) {
		g.w(space)
	}
	g.w(interpClose)
	g.ctx = saved
}

// unbrace spaces an interpolated expression that starts with "{" from "{", a literal "{{" (§2.6).
func (g *gen) unbrace(start int) {
	text := g.out.String()
	if !strings.HasPrefix(text[start:], openBrace) {
		return
	}
	g.out.Reset()
	g.w(text[:start], space, text[start:])
}

// multiline is a multiline string, raw or not: its lines indented by the closing line's prefix.
func (g *gen) multiline(raw bool) {
	prefix := strings.Repeat(indentUnit, g.depth+1)
	if raw {
		g.w(rawPrefix)
	}
	g.w(mlQuote, newline)
	for n := g.r.Intn(maxMLLines + 1); n > 0; n-- {
		g.w(prefix)
		if raw {
			g.w(g.runs(rawRunes))
		} else {
			g.parts()
		}
		g.w(newline)
	}
	g.w(prefix, mlQuote)
}
