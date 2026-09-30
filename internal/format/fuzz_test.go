package format_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/format"
	"github.com/fantasim/canonlang/internal/testkit/golden"
)

// IMPLEMENTATION-PLAN §7.7, FORMATTER.md §1: the layout keeps meaning and comments, idempotent.
func FuzzFormat(f *testing.F) {
	for _, ex := range exampleFiles(f) {
		f.Add(ex.data)
	}
	cases, err := golden.Load("testdata/fmt/*.txtar")
	if err != nil {
		f.Fatal(err)
	}
	for _, c := range cases {
		f.Add(c.Archive.Files[0].Data)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		for _, name := range []string{"a/a.canon", projectFile} {
			in := parse(t, name, data)
			out, err := formatText(t, name, data)
			if errors.Is(err, format.ErrSyntax) {
				if _, err := format.File(in.file); err != nil && !errors.Is(err, format.ErrSyntax) {
					t.Fatalf("%s: %v", name, err)
				}
				continue
			}
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			checkFormatted(t, filepath.ToSlash(name), in, out)
		}
	})
}

// aroundTexts are new texts FuzzRewriteAround starts from: a number, strings short and wide, a
// record written loosely, a list, a comment that ends its line, reserved words and a plain one.
var aroundTexts = append([]string{"7", `"x"`, `"` + strings.Repeat("w", format.Width) + `"`, "{a:1,b:[2,3]}", "[1, 2]", "1 // c"}, wordTexts...)

// FuzzRewriteAround checks that Rewrite, judged around the bytes it changes, is byte for byte
// Rewrite judged on the whole file, for any text of any node.
func FuzzRewriteAround(f *testing.F) {
	// FORMATTER.md §13, IMPLEMENTATION-PLAN §7.7
	inputs := append(exampleFiles(f), example{path: "monster/monster.canon", data: monsterText(f)})
	for i, ex := range inputs {
		for j, text := range aroundTexts {
			f.Add(ex.data, uint16(i*len(aroundTexts)+j), text)
		}
	}
	f.Fuzz(func(t *testing.T, data []byte, pick uint16, text string) {
		out, err := formatText(t, "a/a.canon", data)
		if err != nil {
			return
		}
		tree := parse(t, "a/a.canon", out).file
		all := items(tree)
		if len(all) == 0 {
			return
		}
		changes := []format.Change{{Kind: format.Replace, Node: all[int(pick)%len(all)], Text: text}}
		rewriteChecked(t, "a/a.canon", tree, changes)
	})
}

// FuzzRewrite checks API.md M5 and M6 (IMPLEMENTATION-PLAN §7.7) on one node of a formatted input.
func FuzzRewrite(f *testing.F) {
	for i, ex := range exampleFiles(f) {
		f.Add(ex.data, uint16(i))
	}
	for _, c := range moveCases { // every node of each Move case's commented lists (log-2026-09-29 M4 U1b)
		for i := range items(parse(f, "a/a.canon", []byte(c.src)).file) {
			f.Add([]byte(c.src), uint16(i))
		}
	}
	f.Fuzz(func(t *testing.T, data []byte, pick uint16) {
		out, err := formatText(t, "a/a.canon", data)
		if err != nil {
			return
		}
		ex := example{path: "a/a.canon", data: out}
		tree := parse(t, ex.path, out).file
		all := items(tree)
		if len(all) == 0 {
			return
		}
		n := all[int(pick)%len(all)]
		checkNode(t, ex, tree, n)
		insertCopies(t, ex, tree, n)
		moveCopies(t, ex, tree, n)
	})
}
