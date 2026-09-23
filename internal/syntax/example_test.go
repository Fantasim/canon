package syntax_test

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

const sampleText = "/// Doc.\nconst X = 1 // one\n"

// sampleFile is the parse of sampleText: each token holds the trivia around it, and the tree
// names tokens by index.
func sampleFile() *syntax.File {
	var fs source.FileSet
	src, err := fs.Add("a.canon", "/p/a.canon", []byte(sampleText))
	if err != nil {
		panic(err)
	}
	sp := func(k syntax.TriviaKind, start, end source.Pos) syntax.Trivia {
		return syntax.Trivia{Kind: k, Start: start, End: end}
	}
	space := func(p source.Pos) []syntax.Trivia { return []syntax.Trivia{sp(syntax.TriviaSpace, p, p+1)} }
	toks := []syntax.Token{
		{Kind: syntax.KwConst, Start: 9, End: 14, Trailing: space(14), Leading: []syntax.Trivia{
			sp(syntax.TriviaDocComment, 0, 8), sp(syntax.TriviaNewline, 8, 9),
		}},
		{Kind: syntax.TokIdent, Start: 15, End: 16, Trailing: space(16)},
		{Kind: syntax.TokAssign, Start: 17, End: 18, Trailing: space(18)},
		{Kind: syntax.TokInt, Start: 19, End: 20, Trailing: []syntax.Trivia{
			sp(syntax.TriviaSpace, 20, 21), sp(syntax.TriviaLineComment, 21, 27),
		}},
		{Kind: syntax.TokEOF, Start: 28, End: 28, Leading: []syntax.Trivia{sp(syntax.TriviaNewline, 27, 28)}},
	}
	decl := &syntax.ConstDecl{
		Bounds: syntax.Bounds{From: 0, To: 3},
		Doc:    &syntax.DocComment{Bounds: syntax.Bounds{From: 0, To: 0}, Start: 0, End: 8, Text: "Doc."},
		Name:   &syntax.Ident{Bounds: syntax.Bounds{From: 1, To: 1}, Name: "X"},
		Value:  &syntax.IntLit{Bounds: syntax.Bounds{From: 3, To: 3}, Value: big.NewInt(1)},
	}
	return &syntax.File{
		Bounds: syntax.Bounds{From: 0, To: 4}, Src: src, Tokens: toks,
		FileKind: syntax.FileSource, Decls: []syntax.Decl{decl},
	}
}

// Every node names its first and last tokens: the file gives its span, text and comments.
func Example() {
	f := sampleFile()
	var nodes []string
	syntax.Inspect(f, func(n syntax.Node) bool {
		if n != nil {
			s := f.Span(n)
			nodes = append(nodes, fmt.Sprintf("%s %q", n.Kind(), f.Src.Content[s.Start:s.End]))
		}
		return true
	})
	c := f.Trailing(f.Decls[0])[1]
	fmt.Println(strings.Join(nodes, ", "))
	fmt.Printf("trailing %q\n", f.Src.Content[c.Start:c.End])
	// Output:
	// File "/// Doc.\nconst X = 1 // one\n", ConstDecl "const X = 1", DocComment "/// Doc.", Ident "X", IntLit "1"
	// trailing "// one"
}
