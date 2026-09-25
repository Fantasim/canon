package progen_test

import (
	"bytes"
	"fmt"
	"slices"

	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/testkit/progen"
)

const (
	metaCommentText = "// progen metamorphic\n"
	metaIndent      = "  "
	metaSpace       = " "
)

// gap is a line start of a file that lies outside every token (never inside a multi-line
// string), and whether a doc block reaches it through comments and line breaks only.
type gap struct {
	off      int
	afterDoc bool
}

// gaps are tg's line starts after a line break: an insertion there never makes an NL (GRAMMAR.md §3.1).
func gaps(tg target) []gap {
	if tg.file == nil {
		return nil
	}
	var out []gap
	afterDoc := false
	for _, tok := range tg.file.Tokens {
		afterDoc = scanTrivia(tok.Leading, afterDoc, &out)
		if tok.End > tok.Start {
			afterDoc = false
		}
		afterDoc = scanTrivia(tok.Trailing, afterDoc, &out)
	}
	return out
}

// scanTrivia records the gap after each line break of ts and tells whether a doc block still
// reaches past them: a doc comment starts one, other comments, spaces and breaks keep it.
func scanTrivia(ts []syntax.Trivia, afterDoc bool, out *[]gap) bool {
	for _, tr := range ts {
		switch tr.Kind {
		case syntax.TriviaNewline:
			*out = append(*out, gap{off: int(tr.End), afterDoc: afterDoc})
		case syntax.TriviaDocComment:
			afterDoc = true
		case syntax.TriviaBOM:
			afterDoc = false
		}
	}
	return afterDoc
}

// blankSites add an empty line at a gap no doc block reaches, which it would detach (GRAMMAR.md §9.1).
func blankSites(tg target) []metaSite {
	var out []metaSite
	for _, g := range gaps(tg) {
		if !g.afterDoc {
			out = append(out, triviaSite(tg, g.off, "\n", "blank line"))
		}
	}
	return out
}

// commentSites add a comment line at any gap: a comment keeps a doc block attached (GRAMMAR.md §9.1).
func commentSites(tg target) []metaSite {
	var out []metaSite
	for _, g := range gaps(tg) {
		out = append(out, triviaSite(tg, g.off, metaCommentText, "comment line"))
	}
	return out
}

// spacingSites widen a space run by one or indent a line by two, outside tokens (GRAMMAR.md §2.1).
func spacingSites(tg target) []metaSite {
	var out []metaSite
	for _, g := range gaps(tg) {
		out = append(out, triviaSite(tg, g.off, metaIndent, "indentation"))
	}
	if tg.file == nil {
		return out
	}
	for _, tok := range tg.file.Tokens {
		for _, tr := range slices.Concat(tok.Leading, tok.Trailing) {
			if tr.Kind == syntax.TriviaSpace {
				out = append(out, triviaSite(tg, int(tr.Start), metaSpace, "wider space"))
			}
		}
	}
	return out
}

// triviaSite inserts text at off, described by its line.
func triviaSite(tg target, off int, text, what string) metaSite {
	line := 1 + bytes.Count(tg.src[:off], []byte("\n"))
	return metaSite{edits: []progen.Edit{insert(off, text)}, desc: fmt.Sprintf("%s before line %d", what, line)}
}
