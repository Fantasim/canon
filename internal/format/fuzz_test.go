package format_test

import (
	"errors"
	"path/filepath"
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

// FuzzRewrite checks API.md M5 and M6 (IMPLEMENTATION-PLAN §7.7) on one node of a formatted input.
func FuzzRewrite(f *testing.F) {
	for i, ex := range exampleFiles(f) {
		f.Add(ex.data, uint16(i))
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
	})
}
