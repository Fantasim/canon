package cppgen

import (
	"fmt"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
)

// recordDecoder writes a record's or case's detail::Decode: whole, or, when its body passes the budget, cut between whole units into Decode<N> parts of <P>Access that it calls in order (CODEGEN.md §2.11, §7.6; DECISIONS 435); a cut decoder is kept apart, to follow the access struct that declares its parts (§2.7). A unit is a field with its built default, path, checks and define lookup, or a stored fn.
func (g *gen) recordDecoder(c class) {
	name := g.className(c)
	fields, fns := c.shape()
	head := g.capture(func() { g.decodeHead(c, fields, fns) })
	var units []string
	var lines []int
	for _, unit := range g.decodeUnits(fields, fns) {
		text := g.capture(unit)
		units, lines = append(units, text), append(lines, strings.Count(text, newline))
	}
	ends := decodeEnds(lines, strings.Count(head, newline)+decodeTailLines, g.partLines)
	if ends == nil {
		g.c.printf(decodeOpenFormat, name)
		g.c.write(head)
		g.c.write(strings.Join(units, ""))
		g.decoderClose()
		return
	}
	g.cutDecoders += g.capture(func() { g.cutDecoder(name, head, units, ends) })
}

// cutDecoder writes the Decode<N> parts of a decoder, each a definition of <P>Access, then the Decode calling them in order.
func (g *gen) cutDecoder(name, head string, units []string, ends []int) {
	var calls writer
	start := 0
	for n, end := range ends {
		part := ir.CppDecodePart(n)
		g.partDecls = append(g.partDecls, fmt.Sprintf(decodePartDeclFormat, part, name))
		g.c.printf(decodePartOpenFormat, g.pl.AccessName(), part, name)
		g.c.write(strings.Join(units[start:end], ""))
		g.c.linef(1, returnTrue)
		g.c.line(closeBrace)
		g.c.blank()
		calls.linef(1, decodePartCallFormat, g.pl.AccessName(), part)
		start = end
	}
	g.c.printf(decodeOpenFormat, name)
	g.c.write(head)
	g.c.write(calls.String())
	g.decoderClose()
}

// decodeEnds are the ends of the parts of a decoder whose units hold lines, or nil for none: a decoder whose units write nothing (every field an input) has nothing to cut, and a part of no statement would leave its parameters unused.
func decodeEnds(lines []int, whole, limit int) []int {
	if !slices.ContainsFunc(lines, func(n int) bool { return n > 0 }) {
		return nil
	}
	return ir.PartEnds(lines, limit, whole, decodePartTailLines)
}
