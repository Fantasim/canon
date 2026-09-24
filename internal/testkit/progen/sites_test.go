package progen_test

import (
	"bytes"
	"strings"

	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/testkit/progen"
)

// nodes lists every node of type T in the target's tree, in source order.
func nodes[T syntax.Node](tg target) []T {
	var out []T
	if tg.file == nil {
		return nil
	}
	syntax.Inspect(tg.file, func(n syntax.Node) bool {
		if v, ok := n.(T); ok && n != nil && n.Last() >= n.First() {
			out = append(out, v)
		}
		return true
	})
	return out
}

// span is a node's byte range.
func span(tg target, n syntax.Node) (int, int) {
	s := tg.file.Span(n)
	return int(s.Start), int(s.End)
}

// tokSpan is token i's byte range.
func tokSpan(tg target, i syntax.Tok) (int, int) {
	t := tg.file.Tokens[i]
	return int(t.Start), int(t.End)
}

// text is the source of a node.
func text(tg target, n syntax.Node) string {
	s, e := span(tg, n)
	return string(tg.src[s:e])
}

// isSource tells a target that is a package source file (not a layer or a translation).
func isSource(tg target) bool { return tg.file != nil && tg.file.FileKind == syntax.FileSource }

func isLayer(tg target) bool { return tg.file != nil && tg.file.FileKind == syntax.FileLayer }

func isProject(tg target) bool { return tg.path == projectFile }

// mark is an edit that changes nothing and marks [s, e) as where the finding starts.
func mark(tg target, s, e int) progen.Edit {
	return progen.Edit{Start: s, End: e, Text: string(tg.src[s:e])}
}

func insert(at int, text string) progen.Edit { return progen.Edit{Start: at, End: at, Text: text} }

func replace(s, e int, text string) progen.Edit { return progen.Edit{Start: s, End: e, Text: text} }

// keeping marks the nodes a site copies or relies on, so that shrinking keeps them too.
func keeping(tg target, s progen.Site, ns ...syntax.Node) progen.Site {
	for _, n := range ns {
		a, b := span(tg, n)
		s.Edits = append(s.Edits, mark(tg, a, b))
	}
	return s
}

// site is a site whose first edit is the focus.
func site(focus progen.Edit, more ...progen.Edit) progen.Site {
	return progen.Site{Edits: append([]progen.Edit{focus}, more...)}
}

// seq is a site of edits given in text order (edits at one offset apply in that order), the
// focus being edits[focus].
func seq(focus int, edits ...progen.Edit) progen.Site {
	return progen.Site{Edits: edits, Focus: focus}
}

// sitesOf maps every node of type T through f, dropping the nodes f refuses.
func sitesOf[T syntax.Node](tg target, keep func(T) bool, f func(T) progen.Site) []progen.Site {
	var out []progen.Site
	for _, n := range nodes[T](tg) {
		if keep == nil || keep(n) {
			out = append(out, f(n))
		}
	}
	return out
}

// lineEnd is the offset of the line break ending the line of off (or the end of the text).
func lineEnd(tg target, off int) int {
	if i := bytes.IndexByte(tg.src[off:], '\n'); i >= 0 {
		return off + i
	}
	return len(tg.src)
}

// lineStart is the offset of the first byte of the line of off.
func lineStart(tg target, off int) int { return bytes.LastIndexByte(tg.src[:off], '\n') + 1 }

// indent is the leading whitespace of the line of off.
func indent(tg target, off int) string {
	s := lineStart(tg, off)
	line := tg.src[s:lineEnd(tg, off)]
	return string(line[:len(line)-len(bytes.TrimLeft(line, " \t"))])
}

// endsLine tells a node that is the last thing on its line, comments aside.
func endsLine(tg target, n syntax.Node) bool {
	_, e := span(tg, n)
	rest := strings.TrimLeft(string(tg.src[e:lineEnd(tg, e)]), " \t")
	return rest == "" || strings.HasPrefix(rest, "//")
}

// startsLine tells a node that is the first thing on its line.
func startsLine(tg target, n syntax.Node) bool {
	s, _ := span(tg, n)
	return strings.TrimSpace(string(tg.src[lineStart(tg, s):s])) == ""
}

// declEnd is the end of the file's text, where a new declaration is appended.
func declEnd(tg target) int { return len(bytes.TrimRight(tg.src, " \t\n")) }

// appendDecl is a site adding a declaration at the end of the file: before + focus + after.
func appendDecl(tg target, before, focus, after string) progen.Site {
	end := declEnd(tg)
	return seq(1, insert(end, "\n\n"+before), insert(end, focus), insert(end, after+"\n"))
}

// statementSites insert a statement as the first of a block, on its own line: focus is the
// statement's text.
func statementSites(tg target, stmt string, keep func(*syntax.Block) bool) []progen.Site {
	var out []progen.Site
	for _, b := range nodes[*syntax.Block](tg) {
		if keep != nil && !keep(b) || len(b.Stmts) == 0 || !startsLine(tg, b.Stmts[0]) {
			continue
		}
		s, _ := span(tg, b.Stmts[0])
		out = append(out, seq(0, insert(s, stmt), insert(s, "\n"+indent(tg, s))))
	}
	return out
}
