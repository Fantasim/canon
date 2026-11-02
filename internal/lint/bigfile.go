package lint

import (
	"bytes"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/syntax"
)

// bigFiles reports W0007 (API.md L8): a file over maxFileLines lines whose one longest declaration is a sound let.
func bigFiles(p *pass, pkg *check.Package, bag *diag.Bag) {
	for _, f := range sources(pkg) {
		lines := lineCount(f.Src.Content)
		if lines <= maxFileLines {
			continue
		}
		longest, declLines, unique := longestDecl(f)
		let, isLet := longest.(*syntax.LetDecl)
		if unique && isLet && let.Name != nil && p.declared(let.Name) != nil {
			diag.W0007.At(f.Span(let.Name), int64(lines), let.Name.Name, int64(declLines)).Report(bag)
		}
	}
}

// lineCount is the lines of content: its newlines, and one more when the last line has none.
func lineCount(content []byte) int {
	n := bytes.Count(content, []byte{'\n'})
	if len(content) > 0 && content[len(content)-1] != '\n' {
		n++
	}
	return n
}

// longestDecl is the top-level declaration of f over the most lines, with its lines, and false when another ties it.
func longestDecl(f *syntax.File) (syntax.Decl, int, bool) {
	var longest syntax.Decl
	most, ties := 0, false
	for _, d := range f.Decls {
		span := f.Span(d)
		first, _ := f.Src.Position(span.Start)
		last, _ := f.Src.Position(max(span.End-1, span.Start))
		if n := last - first + 1; n > most {
			longest, most, ties = d, n, false
		} else if n == most {
			ties = true
		}
	}
	return longest, most, !ties
}
