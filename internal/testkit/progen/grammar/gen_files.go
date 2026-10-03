package grammar

import (
	"strings"

	"github.com/fantasim/canonlang/internal/syntax"
)

// fileHeader opens a layer or translation file: no doc block before package (GRAMMAR.md §5.2).
func (g *gen) fileHeader() {
	if g.r.OneIn(oneIn4) {
		g.ownLineComment()
	}
	g.tok(syntax.KwPackage)
	g.w(space, g.pick(pkgNames), newline)
}

// layerFile is packageClause NL "layer" IDENT NL { amendDecl NL } (GRAMMAR.md §5.7).
func (g *gen) layerFile() {
	g.fileHeader()
	g.tok(syntax.KwLayer)
	g.w(space, g.lower(), newline)
	for n := g.r.Intn(maxAmends + 1); n > 0 && g.b.Spend(); n-- {
		g.w(newline)
		if g.r.OneIn(oneIn6) {
			g.ownLineComment()
		}
		g.amendDecl()
		g.trailingNote(oneIn8)
		g.w(newline)
	}
	g.maybeNoFinalNL()
}

// maybeNoFinalNL ends the file without its last line break now and then (GRAMMAR.md §5.2: NL | EOF).
func (g *gen) maybeNoFinalNL() {
	if !g.r.OneIn(oneIn6) {
		return
	}
	s := strings.TrimSuffix(g.out.String(), newline)
	g.out.Reset()
	g.out.WriteString(s)
}

// trailingNote ends the line with a comment, one time in n.
func (g *gen) trailingNote(n int) {
	if !g.r.OneIn(n) {
		return
	}
	if g.r.OneIn(oneIn2) {
		g.w(space, blockOpen, g.note(), blockClose)
		return
	}
	g.w(space, linePrefix, g.note())
}

// item writes the body item f, with a blank line, comments and a trailing comment when each
// item has its own line.
func (g *gen) item(f func()) {
	if g.lines && g.r.OneIn(oneIn8) {
		g.w(newline)
	}
	if g.lines && g.r.OneIn(oneIn6) {
		g.ownLineComment()
	}
	f()
	if g.lines {
		g.trailingNote(oneIn6)
	}
}

// amendDecl is { DOC } "amend" IDENT BraceList( amendItem ).
func (g *gen) amendDecl() {
	if g.r.OneIn(oneIn3) {
		g.doc()
	}
	g.tok(syntax.KwAmend)
	g.w(space)
	if g.r.OneIn(oneIn2) {
		g.w(g.lower())
	} else {
		g.w(g.upper())
	}
	g.w(space)
	g.body(func() { g.item(g.amendItem) })
}

// amendItem is { DOC } amendPath ":" expr.
func (g *gen) amendItem() {
	if g.lines && g.r.OneIn(oneIn3) {
		g.doc()
	}
	g.w(g.pick(pathHeads))
	for n := g.r.Intn(maxKeySegs); n > 0; n-- {
		g.amendSegment()
	}
	g.w(colonSpace)
	g.expr(0)
}

// amendSegment is "." WORD, "[" expr "]" or "[#" INT "]".
func (g *gen) amendSegment() {
	switch {
	case g.r.OneIn(oneIn3):
		g.w(dotSep, g.pick(wordPool))
	case g.r.OneIn(oneIn2):
		g.w(hashOpen, g.pick(positions), closeBrack)
	default:
		g.index()
	}
}

// translationFile is packageClause NL "translation" IDENT NL { translationEntry NL }.
func (g *gen) translationFile() {
	g.fileHeader()
	g.tok(syntax.KwTranslation)
	g.w(space, g.pick(langCodes), newline)
	for n := g.r.Intn(maxEntries + 1); n > 0 && g.b.Spend(); n-- {
		if g.r.OneIn(oneIn6) {
			g.ownLineComment()
		}
		if g.r.OneIn(oneIn4) {
			g.doc()
		}
		g.translationEntry()
		g.trailingNote(oneIn6)
		g.w(newline)
		if g.r.OneIn(oneIn4) {
			g.w(newline)
		}
	}
	g.maybeNoFinalNL()
}

// translationEntry is a key WORD { "." WORD } and a string literal, one space apart.
func (g *gen) translationEntry() {
	g.w(g.pick(wordPool))
	for n := g.r.Intn(maxKeySegs); n > 0; n-- {
		g.w(dotSep, g.pick(wordPool))
	}
	g.w(space)
	g.str()
}

// projectFile is { DOC } "project" IDENT BraceList( projectItem ) (GRAMMAR.md §7).
func (g *gen) projectFile() {
	if g.r.OneIn(oneIn4) {
		g.ownLineComment()
	}
	if g.r.OneIn(oneIn3) {
		g.doc()
	}
	g.tok(syntax.KwProject)
	g.w(space, g.lower(), space)
	g.bodyList(func() { g.item(g.projectItem) }, true)
	g.w(newline)
}

// projectItem is { DOC } WORD ( ":" pValue | BraceList( pEntry ) ).
func (g *gen) projectItem() {
	if g.lines && g.r.OneIn(oneIn3) {
		g.doc()
	}
	g.w(g.pick(projectKeys))
	if g.r.OneIn(oneIn3) {
		g.w(space)
		g.projectMap()
		return
	}
	g.w(colonSpace)
	g.projectValue()
}

// projectMap is BraceList( pEntry ), with a trailing separator now and then.
func (g *gen) projectMap() { g.bodyList(func() { g.item(g.projectPair) }, true) }

// projectPair is { DOC } ( WORD | stringLit ) ":" pValue.
func (g *gen) projectPair() {
	if g.lines && g.r.OneIn(oneIn3) {
		g.doc()
	}
	if g.r.OneIn(oneIn3) {
		g.w(quote, g.runs(textRunes), quote)
	} else {
		g.w(g.pick(dataWords))
	}
	g.w(colonSpace)
	g.projectValue()
}

// projectValue is a constant string, an INT, a qualified name, a list or a map of pairs.
func (g *gen) projectValue() {
	if !g.b.Enter() {
		g.w(g.pick(ints))
		return
	}
	defer g.b.Leave()
	projectValues[g.r.Intn(len(projectValues))](g)
}

// projectList is "[" [ pValue { "," pValue } [ "," ] ] "]".
func (g *gen) projectList() {
	g.w(openBrack)
	g.with(0, func() {
		n := g.r.Intn(maxValues + 1)
		g.list(n, g.projectValue)
		if n > 0 && g.r.OneIn(oneIn4) {
			g.w(",")
		}
	})
	g.w(closeBrack)
}
