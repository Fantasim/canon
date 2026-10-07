package check

import (
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
)

// IMPLEMENTATION-PLAN §7.6, NFR-02: a journaled finding reads the files of its related notes and stack frames too.
func TestReadsOfListsRelatedAndStackFiles(t *testing.T) {
	put := func(bag *diag.Bag) {
		diag.E5001.At(source.Span{File: 1}, "m").
			Related(source.Span{File: 2}, diag.NoteCheck("c")).
			Stack([]diag.Frame{{Fn: "f", Span: source.Span{File: 3}}}).Report(bag)
	}
	if got, want := readsOf("p", put), []source.FileID{1, 2, 3}; !slices.Equal(got, want) {
		t.Errorf("reads %v, want %v", got, want)
	}
}
