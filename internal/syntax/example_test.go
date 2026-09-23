package syntax_test

import (
	"fmt"
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

const sampleText = "/// Doc.\npackage a\n\nconst X = -1 // one\n"

// sampleFile is the parse of sampleText.
func sampleFile() *syntax.File {
	var fs source.FileSet
	src, err := fs.Add("a/a.canon", "/p/a/a.canon", []byte(sampleText))
	if err != nil {
		panic(err)
	}
	return syntax.Parse(src, syntax.FileSource, diag.NewBag(&fs, "a"))
}

// Parse builds the tree and the lossless token stream: every node names its first and last
// tokens, so the file gives its span, its text and its comments.
func Example() {
	var fs source.FileSet
	src, err := fs.Add("a/a.canon", "/p/a/a.canon", []byte(sampleText))
	if err != nil {
		fmt.Println(err)
		return
	}
	bag := diag.NewBag(&fs, "a")
	f := syntax.Parse(src, syntax.FileSource, bag)
	var nodes []string
	syntax.Inspect(f, func(n syntax.Node) bool {
		if n != nil && n != syntax.Node(f) {
			s := f.Span(n)
			nodes = append(nodes, fmt.Sprintf("%s %q", n.Kind(), f.Src.Content[s.Start:s.End]))
		}
		return true
	})
	decl := f.Decls[0]
	c := f.Trailing(decl)[1]
	fmt.Println(strings.Join(nodes, ", "))
	fmt.Printf("doc %q, trailing %q, %d findings, %s nameable %v\n", f.Doc.Text, f.Src.Content[c.Start:c.End],
		len(bag.Findings()), syntax.LookupWord("check"), syntax.IsNameable(syntax.KwCheck))
	// Output:
	// DocComment "/// Doc.", QualifiedName "a", Ident "a", ConstDecl "const X = -1", Ident "X", IntLit "-1"
	// doc "Doc.", trailing "// one", 0 findings, check nameable true
}
