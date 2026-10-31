package cppgen

import "strings"

// doc writes a doc comment at depth (CODEGEN.md §2.6): `/// ` + line, `///` for an empty line; the block form when a line ends in a backslash, and `??/` written `?\?/` in both (DECISIONS 423, pending).
func (g *gen) doc(depth int, text string) {
	if text == "" {
		return
	}
	text = strings.ReplaceAll(text, trigraphSlash, trigraphSlashSafe)
	if endsInSplice(text) {
		g.docBlock(depth, text)
		return
	}
	for l := range strings.SplitSeq(text, newline) {
		if l == "" {
			g.h.linef(depth, docEmpty)
			continue
		}
		g.h.linef(depth, docLineFormat, l)
	}
}

// docBlock writes text as `/**`, ` * ` + line (` *` for an empty line), ` */`, with a backslash between every adjacent `/` and `*`: a backslash ending a line only joins two comment lines there, and no generated line is spliced into the comment (CODEGEN.md §2.6, CG-06; DECISIONS 423, pending).
func (g *gen) docBlock(depth int, text string) {
	g.h.linef(depth, docBlockOpen)
	for l := range strings.SplitSeq(splitDelimiters(text), newline) {
		if l == "" {
			g.h.linef(depth, docBlockEmpty)
			continue
		}
		g.h.linef(depth, docBlockLineFormat, l)
	}
	g.h.lineAt(depth, space+commentEnd)
}

// endsInSplice reports whether a line of text ends in a backslash, spaces and tabs after it included: C++ would join the next line to it.
func endsInSplice(text string) bool {
	for l := range strings.SplitSeq(text, newline) {
		if strings.HasSuffix(strings.TrimRight(l, spliceBlanks), lineSplice) {
			return true
		}
	}
	return false
}

// splitDelimiters is text with a backslash between every `/` and `*` that are adjacent, in either order, scanned left to right: `*/` is `*\/`, `/*` is `/\*`, `/*/` is `/\*\/`, so a block comment's text holds no delimiter.
func splitDelimiters(text string) string {
	var b strings.Builder
	for i := range len(text) {
		prev := byte(0)
		if i > 0 {
			prev = text[i-1]
		}
		if (prev == slashByte && text[i] == starByte) || (prev == starByte && text[i] == slashByte) {
			b.WriteString(lineSplice)
		}
		b.WriteByte(text[i])
	}
	return b.String()
}
