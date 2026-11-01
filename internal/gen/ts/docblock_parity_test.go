package tsgen_test

import (
	"testing"

	tsgen "github.com/fantasim/canonlang/internal/gen/ts"
	"github.com/fantasim/canonlang/internal/testkit/starblock"
)

// CODEGEN.md §2.6: the star-line block of a doc text is the one gen/cpp writes (the corpus is shared with gen/cpp's test).
func TestDocCommentMatchesTheSharedCorpus(t *testing.T) {
	t.Parallel()
	for _, c := range starblock.Cases() {
		if got := tsgen.DocCommentText(c.Doc); got != c.Block {
			t.Errorf("%s: docComment(%q) = %q, want %q", c.Name, c.Doc, got, c.Block)
		}
	}
}
