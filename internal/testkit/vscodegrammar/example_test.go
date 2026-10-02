package vscodegrammar_test

import (
	"fmt"

	"github.com/fantasim/canonlang/internal/testkit/vscodegrammar"
)

// A line of Canon, tokenised by the grammar under test.
func Example() {
	g, err := vscodegrammar.Load("../../../editors/vscode/syntaxes/canon.tmLanguage.json")
	if err != nil {
		fmt.Println(err)
		return
	}
	out, err := g.Tokenize("let 90s")
	if err != nil {
		fmt.Println(err)
		return
	}
	fmt.Print(out)
	// Output:
	// 1:0-3 "let" storage.type.canon
	// 1:4-7 "90s" constant.numeric.duration.canon
}
