package syntax_test

import (
	"bytes"
	"path"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/testkit/golden"
)

const astSection = "ast"

// GRAMMAR.md §5: each case prints its files' trees and findings into its ast section.
func TestParseCases(t *testing.T) {
	golden.Run(t, "testdata/parse/*.txtar", func(t *testing.T, c golden.Case) []byte {
		t.Helper()
		var out strings.Builder
		for _, f := range c.Archive.Files {
			if path.Ext(f.Name) != ".canon" {
				continue
			}
			tree, bag, fs := parseText(t, f.Name, f.Data)
			checkTree(t, f.Name, tree, len(bag.Findings()) == 0)
			if !bytes.Equal(printTokens(tree), tree.Src.Content) {
				t.Errorf("%s: tokens and trivia do not reproduce the file", f.Name)
			}
			out.WriteString("== " + f.Name + "\n" + dumpFile(tree) + findingsSep + renderFindings(t, bag, fs))
		}
		return []byte(out.String())
	}, golden.Expected(astSection))
}
