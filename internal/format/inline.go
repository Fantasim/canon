package format

import (
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// broken reports a list whose closing bracket and first item start their lines.
func (b *builder) broken(l list) bool {
	if b.oneLine(l.open, l.close) || !b.ownLine(int(b.f.Tokens[l.close].Start)) {
		return false
	}
	return len(l.items) == 0 || b.ownLine(b.ownsItem(l.items[0]).lo)
}

// ownLine reports an offset with only indentation before it on its line.
func (b *builder) ownLine(at int) bool {
	src := b.f.Src.Content
	return strings.TrimLeft(string(src[lineStart(src, at):at]), space) == ""
}

// separator is the last token of item n with its separator: the comma after it, if any.
func (b *builder) separator(n syntax.Node) syntax.Tok {
	if c := b.after(n.Last()); b.f.Tokens[c].Kind == syntax.TokComma {
		return c
	}
	return n.Last()
}

// indentOf is the indentation of the line holding token t.
func (b *builder) indentOf(t syntax.Tok) int {
	src := b.f.Src.Content
	start := lineStart(src, int(b.f.Tokens[t].Start))
	line := string(src[start:lineEnd(src, start)])
	return len(line) - len(strings.TrimLeft(line, space))
}

// kindsOf are the kinds of the tokens of t, read by the lexer; nil when it holds none.
func kindsOf(t string) []syntax.TokenKind {
	var fs source.FileSet
	src, err := fs.Add(fragmentPath, fragmentPath, []byte(t))
	if err != nil {
		return nil
	}
	var kinds []syntax.TokenKind
	for _, tk := range syntax.Parse(src, syntax.FileSource, diag.NewBag(&fs, "")).Tokens {
		if tk.Kind != syntax.TokBOF && tk.Kind != syntax.TokEOF && tk.Kind != syntax.TokNL {
			kinds = append(kinds, tk.Kind)
		}
	}
	return kinds
}

// Continues reports text t, opening an item of kind item, starting with a token that continues
// the line above it (DECISIONS 459).
func Continues(t string, item syntax.NodeKind) bool {
	kinds := kindsOf(t)
	return len(kinds) > 0 && forJoins(kinds[0], item)
}

// edges are whether the first token of t continues the line above it and the kind of its last.
func edges(t string) (joins bool, last syntax.TokenKind) {
	kinds := kindsOf(t)
	if len(kinds) == 0 {
		return false, syntax.TokInvalid
	}
	var item syntax.NodeKind
	if len(kinds) > 1 {
		item = forItemAfter[kinds[1]]
	}
	return forJoins(kinds[0], item), namedKind(kinds)
}

// namedKind is the kind ending kinds; a word after "." is a name (DECISIONS 302).
func namedKind(kinds []syntax.TokenKind) syntax.TokenKind {
	n := len(kinds) - 1
	if n > 0 && (kinds[n-1] == syntax.TokDot || kinds[n-1] == syntax.TokOptDot) {
		return syntax.TokIdent
	}
	return kinds[n]
}
