package edit

import (
	"cmp"

	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
)

// stated is how a found value is listed (log-2026-09-29 M4 U4a): kept when data or a let's or
// an entry's value states it or code computes it; computed unless its span is a name or key
// token, which a rename rewrites and whose name is no other reference.
func (sc *refScan) stated(p *value.Prov) (keep, computed bool) {
	switch p.Kind {
	case value.ProvJSON, value.ProvCSV, value.ProvDefines, value.ProvText:
		return true, !sc.dataToken(p.Span)
	case value.ProvComputed:
		return true, true
	case value.ProvLiteral:
		if !sc.s.inLetValue(p.Span) {
			return false, false
		}
		return true, !sc.token(p.Span)
	default:
		return false, false
	}
}

// token reports a span that is a name or key token of its file's syntax.
func (sc *refScan) token(sp source.Span) bool {
	spans, ok := sc.tokens[sp.File]
	if !ok {
		spans = tokenSpans(sc.s.files[sp.File])
		sc.tokens[sp.File] = spans
	}
	return spans[sp]
}

// tokenSpans are the spans of a file's names and key literals: identifiers, selectors and their
// names, string and integer literals.
func tokenSpans(f *syntax.File) map[source.Span]bool {
	out := map[source.Span]bool{}
	if f == nil {
		return out
	}
	syntax.Inspect(f, func(n syntax.Node) bool {
		switch n.(type) {
		case *syntax.IdentExpr, *syntax.Ident, *syntax.SelectorExpr, *syntax.StringLit, *syntax.RawStringLit, *syntax.IntLit:
			out[f.Span(n)] = true
		}
		return true
	})
	return out
}

// dataToken reports a data file's span that is a string, a key or a number, not an object or
// an array (an entry a ref was converted from).
func (sc *refScan) dataToken(sp source.Span) bool {
	content := sc.s.a.Files().Content(sp.File)
	if int(sp.Start) >= len(content) {
		return false
	}
	c := content[sp.Start]
	return c != jsonOpen && c != bracketOpen
}

// inLetValue reports a span inside the declaration of a let or an entry.
func (s *Snapshot) inLetValue(sp source.Span) bool {
	f := s.files[sp.File]
	if f == nil {
		return false
	}
	for _, d := range f.Decls {
		switch d.(type) {
		case *syntax.LetDecl, *syntax.EntryDecl:
			if contains(f.Span(d), sp) {
				return true
			}
		}
	}
	return false
}

// spanOrder orders spans by file, then start.
func spanOrder(a, b source.Span) int {
	return cmp.Or(cmp.Compare(a.File, b.File), cmp.Compare(a.Start, b.Start))
}
