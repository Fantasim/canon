package grammar

import (
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/syntax"
)

// ctx is what the enclosing constructs allow (GRAMMAR.md §5.10, §6.1).
type ctx uint8

// Rand is the source of a generator's choices (progen.Rand).
type Rand interface {
	Intn(n int) int
	OneIn(n int) bool
}

// Budget bounds a generated file (progen.Budget): a node taken, a level entered and left.
type Budget interface {
	Spend() bool
	Enter() bool
	Leave()
}

// pickOf is an element of xs chosen by r.
func pickOf[T any](r Rand, xs []T) T { return xs[r.Intn(len(xs))] }

// gen writes one source file.
type gen struct {
	r      Rand
	b      Budget
	out    strings.Builder
	depth  int
	ctx    ctx
	lines  bool // the items being written each stand on their own line: they may have docs
	fnType bool // the last primType written was a function type
	notes  int  // comments written so far: each gets its own text, so a moved one shows
}

// Generate is a source file that parses without a finding (GRAMMAR.md §5).
func Generate(r Rand, b Budget) []byte { return GenerateKind(r, b, Source) }

// GenerateKind is a file of kind k that parses without a finding (GRAMMAR.md §5.2, §5.7, §7).
func GenerateKind(r Rand, b Budget, k Kind) []byte {
	g := &gen{r: r, b: b}
	fileKinds[k](g)
	return []byte(g.out.String())
}

func (g *gen) w(parts ...string) {
	for _, p := range parts {
		g.out.WriteString(p)
	}
}

func (g *gen) tok(k syntax.TokenKind) { g.w(k.String()) }

// ownLineComment writes a line or block comment on its own line.
func (g *gen) ownLineComment() {
	if g.r.OneIn(oneIn2) {
		g.w(blockOpen, g.note(), blockClose, newline)
		return
	}
	g.w(linePrefix, g.note(), newline)
}

// note is a comment's text, never written before in the file.
func (g *gen) note() string {
	g.notes++
	return notePrefix + strconv.Itoa(g.notes)
}

// nl ends the line and indents the next one.
func (g *gen) nl() {
	g.w(newline, strings.Repeat(indentUnit, g.depth))
}

// sp is a space between tokens, now and then two: the layout is the formatter's business.
func (g *gen) sp() {
	g.w(space)
	if g.r.OneIn(oneIn8) {
		g.w(space)
	}
}

func (g *gen) pick(xs []string) string { return pickOf(g.r, xs) }

func (g *gen) lower() string { return g.pick(lowerNames) }

func (g *gen) upper() string { return g.pick(upperNames) }

// with runs f with the context flags c set, and header mode cleared unless c sets it.
func (g *gen) with(c ctx, f func()) {
	saved := g.ctx
	g.ctx = g.ctx&^(inHeader|inClosed) | c
	f()
	g.ctx = saved
}

// closed runs f where no open range may end the expression, a word following it (§5.11).
func (g *gen) closed(f func()) {
	saved := g.ctx
	g.ctx |= inClosed
	f()
	g.ctx = saved
}

// bracketed runs f between open and close, header mode off inside (GRAMMAR.md §6.1).
func (g *gen) bracketed(open, close string, f func()) {
	g.w(open)
	g.with(0, f)
	g.w(close)
}

func (g *gen) file() {
	if g.r.OneIn(oneIn3) {
		g.doc()
	}
	g.tok(syntax.KwPackage)
	g.w(space, g.pick(pkgNames), newline)
	for n := g.r.Intn(maxImports); n > 0; n-- {
		g.importDecl()
	}
	for n := 0; n < maxDecls && g.b.Spend(); n++ {
		g.w(newline)
		if g.r.OneIn(oneIn6) {
			g.ownLineComment()
		}
		g.topDecl()
		g.w(newline)
	}
}

// doc writes a doc block, attached to what follows on the next line.
func (g *gen) doc() {
	for n := 1 + g.r.Intn(maxDocLines); n > 0; n-- {
		g.w(pickOf(g.r, docOpeners), g.note())
		g.nl()
	}
}

func (g *gen) importDecl() {
	g.tok(syntax.KwImport)
	g.w(space, g.pick(pkgNames))
	if g.r.OneIn(oneIn3) {
		g.w(space)
		g.tok(syntax.KwAs)
		g.w(space, g.lower())
	}
	if g.r.OneIn(oneIn2) {
		g.w(space, braceSpaced, g.upper(), commaSpace, g.upper()+"x", spacedBrace)
	}
	g.w(newline)
}

// annotations writes up to two distinct annotations of pool, each between before and after.
func (g *gen) annotations(pool []string, before, after string) {
	if g.r.OneIn(oneIn2) {
		return
	}
	first := g.r.Intn(len(pool))
	g.w(before, pool[first], after)
	if second := g.r.Intn(len(pool)); second != first && g.r.OneIn(oneIn3) {
		g.w(before, pool[second], after)
	}
}

// suffix writes annotations after an item: " @a @b".
func (g *gen) suffix(pool []string) { g.annotations(pool, space, "") }

// prefixAnnotations writes annotations of pool on their own lines before a declaration.
func (g *gen) prefixAnnotations(pool []string) {
	if g.r.OneIn(oneIn2) {
		return
	}
	g.w(pool[g.r.Intn(len(pool))])
	g.nl()
}

// list writes n items separated by ", ", each by f.
func (g *gen) list(n int, f func()) {
	for i := range n {
		if i > 0 {
			g.w(commaSpace)
		}
		f()
	}
}

// qualified is a dotted name of upper-case segments.
func (g *gen) qualified() string {
	if g.r.OneIn(oneIn4) {
		return g.lower() + "." + g.upper()
	}
	return g.upper()
}
