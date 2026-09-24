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
