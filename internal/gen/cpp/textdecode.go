package cppgen

import (
	_ "embed"

	"github.com/fantasim/canonlang/internal/ir"
)

//go:embed text/text_decode.txt
var textDecodeText string

// textDeclarations declare Decode<Fn>File of each decoded @text fn in the package namespace: it reads the file's text, keeps the file order of a map, and returns no value unless all of it decodes (CODEGEN.md §5.13, DECISIONS 363).
func (g *gen) textDeclarations() {
	for _, fn := range g.pl.TextDecoders() {
		g.h.linef(0, textDocFormat, fn.Name)
		g.h.linef(0, textDeclFormat, g.storage(ir.TextResult(fn)), g.pl.TextDecoder(fn))
		g.h.blank()
	}
}

// textHelpers are detail's Decode<Fn>File, which read the parsed file as a value of the fn's result type, its paths from `$`.
func (g *gen) textHelpers() {
	for _, fn := range g.pl.TextDecoders() {
		leave := g.enter(g.p.Name + qnameSep + fn.Name)
		t := ir.TextResult(fn)
		g.c.printf(readerOpenFormat, g.pl.TextDecoder(fn), g.storage(t))
		g.decodeValue(1, sourceVar, quote(dollar), leaf{t: t, dst: outVar})
		g.c.linef(1, returnOk)
		g.c.line(closeBrace)
		g.c.blank()
		leave()
	}
}

// textDecoders define Decode<Fn>File: the parse, with the file's name on every error, then detail's reader.
func (g *gen) textDecoders() {
	for _, fn := range g.pl.TextDecoders() {
		g.c.printf(textDecodeText, g.storage(ir.TextResult(fn)), g.pl.TextDecoder(fn), quote(g.pl.TextFile(fn)))
	}
}

// declareTextNames declares each Decode<Fn>File among the namespace's names.
func (g *gen) declareTextNames() {
	for _, fn := range g.pl.TextDecoders() {
		g.declare(g.pl.TextDecoder(fn), fn.Name)
	}
}
