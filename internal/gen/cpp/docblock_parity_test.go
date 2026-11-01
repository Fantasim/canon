package cppgen_test

import (
	"testing"

	cppgen "github.com/fantasim/canonlang/internal/gen/cpp"
	"github.com/fantasim/canonlang/internal/testkit/starblock"
)

// CODEGEN.md §2.6: the star-line block of a doc text is the one gen/ts writes (the corpus is shared with gen/ts's test).
func TestDocBlockMatchesTheSharedCorpus(t *testing.T) {
	t.Parallel()
	for _, c := range starblock.Cases() {
		if got := cppgen.DocBlockText(c.Doc); got != c.Block {
			t.Errorf("%s: docBlock(%q) = %q, want %q", c.Name, c.Doc, got, c.Block)
		}
	}
}
