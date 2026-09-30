package edit_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/format"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

const (
	keptCanon = "/// P.\npackage d\n\n/// A.\nlet a: Int = 1\n"
	keptLoose = "/// P.\npackage d\n/// A.\nlet a:Int=1\n"
	keptTab   = "/// P.\npackage d\n\n/// E.\nenum E {\n\ta\n}\n"
	keptLex   = "/// P.\npackage d\n\n/// A.\nconst a = \"\\q\"\n"
	keptBOM   = "\ufeff" + keptCanon
)

// The layout verdict kept for a file's tree is a fresh check's, asked once or twice, whether the
// text is canonical or not, holds a carriage return, a byte order mark, a tab indentation or an
// error only the lexer reports, or is not the tree's text at all.
func TestM9KeptVerdict(t *testing.T) {
	// API.md M9
	for _, c := range []struct{ name, text, raw string }{
		{"canonical", keptCanon, keptCanon},
		{"not canonical", keptLoose, keptLoose},
		{"carriage returns", keptCanon, strings.ReplaceAll(keptCanon, "\n", "\r\n")},
		{"other bytes", keptCanon, keptLoose},
		{"byte order mark", keptBOM, keptBOM},
		{"byte order mark, tree without", keptCanon, keptBOM},
		{"tab indentation", keptTab, keptTab},
		{"lexer error", keptLex, keptLex},
	} {
		s := open(t, mapFS{"law/project.canon": file(projectCanon), "law/d/d.canon": file(c.text)}, nil, "", "d")
		var fs source.FileSet
		src, err := fs.Add("d/d.canon", "d/d.canon", []byte(c.raw))
		if err != nil {
			t.Fatal(err)
		}
		want, wantErr := format.Source(src, syntax.FileSource, diag.NewBag(&fs, ""))
		for ask := range 2 {
			got, err := edit.CanonicalSource(s.snap, "d/d.canon", []byte(c.raw))
			if !bytes.Equal(got, want) || !errors.Is(err, wantErr) {
				t.Errorf("%s, ask %d: %q, %v; a fresh check gives %q, %v", c.name, ask, got, err, want, wantErr)
			}
		}
	}
}
