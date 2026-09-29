package format

import (
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// Source parses src as a file of the given kind and returns its canonical layout. A file with a
// syntax error is left unchanged: Source returns ErrSyntax and bag holds the errors; a leading
// byte order mark alone is not one, since the layout removes it.
func Source(src *source.File, kind syntax.FileKind, bag *diag.Bag) ([]byte, error) {
	f := syntax.Parse(src, kind, bag)
	if failed(bag, src) {
		return nil, ErrSyntax
	}
	return File(f)
}

// failed reports an error of src in bag other than a leading byte order mark's (§10).
func failed(bag *diag.Bag, src *source.File) bool {
	return slices.ContainsFunc(bag.Findings(), func(fd diag.Finding) bool {
		return fd.Span.File == src.ID && fd.Severity == diag.Error && fd.Code != diag.E1123.Def().Code
	})
}

// File prints f in the canonical layout; ErrSyntax when a node did not parse. An error the
// lexer alone reports leaves a whole tree: Source, which reads the findings, refuses it.
func File(f *syntax.File) ([]byte, error) {
	if !sound(f) {
		return nil, ErrSyntax
	}
	return render(newBuilder(f).file()), nil
}

// Fresh prints a file the edit API creates, whose brace lists have no layout to keep.
func Fresh(f *syntax.File) ([]byte, error) {
	// FORMATTER.md §13 step 7, §6.3
	if !sound(f) {
		return nil, ErrSyntax
	}
	b := newBuilder(f)
	b.fresh = func(syntax.Tok) bool { return true }
	return render(b.file()), nil
}

// sound reports a tree with its header, without a node that did not parse or an empty one.
func sound(f *syntax.File) bool {
	header := f.FileKind != syntax.FileInvalid && (f.Project != nil || f.FileKind != syntax.FileProject && f.Package != nil)
	return header && soundNode(f)
}
