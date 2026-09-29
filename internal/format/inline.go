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

// edges are the kinds of the first and last tokens of t, read by the lexer.
func edges(t string) (first, last syntax.TokenKind) {
	var fs source.FileSet
	src, err := fs.Add(fragmentPath, fragmentPath, []byte(t))
	if err != nil {
		return syntax.TokInvalid, syntax.TokInvalid
	}
	var kinds []syntax.TokenKind
	for _, tk := range syntax.Parse(src, syntax.FileSource, diag.NewBag(&fs, "")).Tokens {
		if tk.Kind != syntax.TokBOF && tk.Kind != syntax.TokEOF && tk.Kind != syntax.TokNL {
			kinds = append(kinds, tk.Kind)
		}
	}
	if len(kinds) == 0 {
		return syntax.TokInvalid, syntax.TokInvalid
	}
	return kinds[0], kinds[len(kinds)-1]
}
