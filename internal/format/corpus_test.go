package format_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/testkit/golden"
)

// FORMATTER.md §12: every testdata/fmt input formats to its want, a fixed point of same meaning.
func TestCorpus(t *testing.T) {
	golden.Run(t, "testdata/fmt/*.txtar", func(t *testing.T, c golden.Case) []byte {
		in := c.Archive.Files[0]
		got, err := formatText(t, in.Name, in.Data)
		if err != nil {
			t.Fatalf("%s: %v", c.Path, err)
		}
		checkFormatted(t, in.Name, parse(t, in.Name, in.Data), got)
		return got
	})
}
