package diag_test

import (
	"bytes"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
)

// DECISIONS 81: *source.FileSet is a diag.Files.
var _ diag.Files = (*source.FileSet)(nil)

// DECISIONS 81: a file set renders a finding exactly as the in-memory Files of the renderer
// tests, in both forms: locations, a declaration's text and a span of no file.
func TestFileSetRendersLikeMemFiles(t *testing.T) {
	var set source.FileSet
	for _, f := range bagFiles {
		if _, err := set.Add(f.Path, "/"+f.Path, []byte(f.Content)); err != nil {
			t.Fatal(err)
		}
	}
	render := func(files diag.Files, form diag.Format) string {
		bag := diag.NewBag(files, "p")
		diag.E5001.At(source.Span{File: 2, Start: 2, End: 3}, "m").
			Related(source.Span{File: 1, Start: 4, End: 5}, diag.NoteCheck("c")).
			Related(source.Span{File: 1, Start: 2, End: 3}, diag.NoteSource(source.Span{File: 1, Start: 2, End: 5})).Report(bag)
		diag.E1004.At(source.Span{}).Report(bag)
		var buf bytes.Buffer
		opt := diag.RenderOptions{Format: form, Summary: bag.Summary(), Golden: true}
		if err := diag.Render(&buf, files, bag.Findings(), opt); err != nil {
			t.Fatal(err)
		}
		return buf.String()
	}
	for _, form := range []diag.Format{diag.FormatText, diag.FormatJSON} {
		if got, want := render(&set, form), render(bagFiles, form); got != want {
			t.Errorf("form %d: file set rendered\n%s\nwant\n%s", form, got, want)
		}
	}
}
