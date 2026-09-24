package format

import (
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// Source parses src as a file of the given kind and returns its canonical layout. A file with a
// syntax error is left unchanged: Source returns ErrSyntax and bag holds the errors; a leading
// byte order mark alone is not one, since the layout removes it.
func Source(src *source.File, kind syntax.FileKind, bag *diag.Bag) ([]byte, error) {
	f := syntax.Parse(src, kind, bag)
	for _, fd := range bag.Findings() {
		if fd.Span.File == src.ID && fd.Severity == diag.Error && fd.Code != diag.E1123.Def().Code {
			return nil, ErrSyntax
		}
	}
	return File(f)
}

// File prints f in the canonical layout; ErrSyntax when a node did not parse. An error the
// lexer alone reports leaves a whole tree: Source, which reads the findings, refuses it.
func File(f *syntax.File) ([]byte, error) {
	if !sound(f) {
		return nil, ErrSyntax
	}
	return render(newBuilder(f).file()), nil
}

// sound reports a tree with its header, without a node that did not parse or an empty one.
func sound(f *syntax.File) bool {
	ok := f.FileKind != syntax.FileInvalid && (f.Project != nil || f.FileKind != syntax.FileProject && f.Package != nil)
	syntax.Inspect(f, func(n syntax.Node) bool {
		ok = ok && (n == nil || !badKinds[n.Kind()] && n.Last() >= n.First())
		return ok
	})
	return ok
}
