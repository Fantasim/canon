package syntax_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// IMPLEMENTATION-PLAN §4.1: the tokens with their trivia reproduce the file byte for byte.
func TestTokensAreLossless(t *testing.T) {
	f := sampleFile()
	var out []byte
	text := func(start, end source.Pos) { out = append(out, f.Src.Content[start:end]...) }
	for _, tok := range f.Tokens {
		for _, tr := range tok.Leading {
			text(tr.Start, tr.End)
		}
		text(tok.Start, tok.End)
		for _, tr := range tok.Trailing {
			text(tr.Start, tr.End)
		}
	}
	if string(out) != sampleText {
		t.Errorf("tokens print %q, want %q", out, sampleText)
	}
}

// FORMATTER.md §8.1: comments lead a node's first token or trail its last one.
func TestFileComments(t *testing.T) {
	f := sampleFile()
	decl := f.Decls[0].(*syntax.ConstDecl)
	lead := f.Leading(decl)
	if len(lead) == 0 || lead[0].Kind != syntax.TriviaDocComment || f.Span(decl.Doc) != (source.Span{File: 1, Start: 0, End: 8}) {
		t.Errorf("doc block not leading the declaration: %+v", lead)
	}
	trail := f.Trailing(decl)
	if len(trail) != len(f.Trailing(decl.Value)) || trail[len(trail)-1].Kind != syntax.TriviaLineComment {
		t.Errorf("trailing comment not on the last token: %+v", trail)
	}
	if got := f.Span(decl); got != (source.Span{File: 1, Start: 9, End: 20}) {
		t.Errorf("Span(decl) = %+v", got)
	}
}

func TestTok(t *testing.T) {
	if syntax.NoTok.Valid() || !syntax.Tok(0).Valid() {
		t.Error("Valid: NoTok must be the only invalid Tok")
	}
	blank := &syntax.Ident{Bounds: syntax.Bounds{From: 2, To: 2}, Name: syntax.Blank}
	if blank.First() != blank.Last() || blank.Name != syntax.TokUnderscore.String() {
		t.Errorf("blank binder %+v", blank)
	}
}

// The zero values are the invalid kinds, and the enums hold distinct values.
func TestSmallEnums(t *testing.T) {
	var tr syntax.Trivia
	var f syntax.File
	if tr.Kind != syntax.TriviaInvalid || f.FileKind != syntax.FileInvalid {
		t.Error("zero values are not the invalid kinds")
	}
	trivia := []syntax.TriviaKind{
		syntax.TriviaInvalid, syntax.TriviaSpace, syntax.TriviaNewline, syntax.TriviaLineComment,
		syntax.TriviaBlockComment, syntax.TriviaDocComment, syntax.TriviaBOM,
	}
	files := []syntax.FileKind{
		syntax.FileInvalid, syntax.FileSource, syntax.FileLayer, syntax.FileTranslation, syntax.FileProject,
	}
	for i := range trivia {
		if int(trivia[i]) != i {
			t.Errorf("trivia kind %d has value %d", i, trivia[i])
		}
	}
	for i := range files {
		if int(files[i]) != i {
			t.Errorf("file kind %d has value %d", i, files[i])
		}
	}
}
