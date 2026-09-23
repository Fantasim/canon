package syntax_test

import (
	"bytes"
	"path"
	"path/filepath"
	"testing"

	"github.com/fantasim/canonlang/internal/source"
	"golang.org/x/tools/txtar"
)

// IMPLEMENTATION-PLAN §7.7: no panic, findings inside the file, lossless tokens, a sound tree.
func FuzzParse(f *testing.F) {
	for _, ex := range examples(f) {
		f.Add(ex.file.Src.Content)
	}
	cases, err := filepath.Glob("testdata/*/*.txtar")
	if err != nil {
		f.Fatal(err)
	}
	for _, c := range cases {
		a, err := txtar.ParseFile(c)
		if err != nil {
			f.Fatal(err)
		}
		for _, file := range a.Files {
			if path.Ext(file.Name) == ".canon" {
				f.Add(file.Data)
			}
		}
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		for _, name := range []string{"a/a.canon", projectFile} {
			tree, bag, _ := parseText(t, name, data)
			if !bytes.Equal(printTokens(tree), tree.Src.Content) {
				t.Fatalf("%s: tokens and trivia do not reproduce the input", name)
			}
			findings := bag.Findings()
			for _, fd := range findings {
				s := fd.Span
				if s.File != tree.Src.ID || s.Start > s.End || s.End > source.Pos(len(tree.Src.Content)) {
					t.Fatalf("%s: %s has span %+v outside the file", name, fd.Code, s)
				}
			}
			checkTree(t, name, tree, len(findings) == 0)
		}
	})
}
