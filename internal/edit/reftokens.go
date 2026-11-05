package edit

import (
	"cmp"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
	"github.com/fantasim/canonlang/internal/value"
)

// stated is how v, of provenance p, is listed (log-2026-09-29 M4 U4a): kept when data or a let's
// or an entry's value states it or code computes it, as a ref converted from an entry or a layer
// line's value its name tokens do not state (API.md R7; DECISIONS 489); computed unless so stated.
func (sc *refScan) stated(v value.Value, p *value.Prov) (keep, computed bool) {
	switch p.Kind {
	case value.ProvJSON, value.ProvCSV, value.ProvDefines, value.ProvText:
		return true, !sc.dataToken(p.Span)
	case value.ProvComputed:
		return true, true
	case value.ProvLiteral, value.ProvLayer:
		_, isRef := v.(*value.Ref)
		nt := sc.notToken(p)
		switch {
		case nt && (isRef || p.Kind == value.ProvLayer):
			return true, true
		case p.Kind == value.ProvLayer || !sc.s.inLetValue(p.Span):
			return false, false
		}
		return true, nt
	default:
		return false, false
	}
}

// notToken reports a source provenance whose span is no key or member token: any expression but
// a name or a literal, an entry read (an expression typed as a record), a let or a field read.
func (sc *refScan) notToken(p *value.Prov) bool {
	f := sc.s.files[p.Span.File]
	if f == nil {
		return false
	}
	spans, ok := sc.tokens[p.Span.File]
	if !ok {
		spans = sc.tokenSpans(f)
		sc.tokens[p.Span.File] = spans
	}
	return !spans[p.Span]
}

// tokenSpans marks f's names and key literals (identifiers, selectors and their names, string and
// integer literals): true for a token, false for an entry, a let or a field it reads and for any
// expression typed as a record, which also holds at a name sharing its span (DECISIONS 489).
func (sc *refScan) tokenSpans(f *syntax.File) map[source.Span]bool {
	out := map[source.Span]bool{}
	syntax.Inspect(f, func(n syntax.Node) bool {
		sp := f.Span(n)
		if _, met := out[sp]; !met {
			if tok, marked := sc.tokenAt(n); marked {
				out[sp] = tok
			}
		}
		return true
	})
	return out
}

// tokenAt is tokenSpans' mark of n, read through its parens, which are transparent (DECISIONS
// 481): whether it is a token, and whether it is marked at all.
func (sc *refScan) tokenAt(n syntax.Node) (tok, marked bool) {
	if e, isExpr := n.(syntax.Expr); isExpr {
		n = syntax.Unparen(e)
	}
	switch n.(type) {
	case *syntax.IdentExpr, *syntax.Ident, *syntax.SelectorExpr, *syntax.StringLit, *syntax.RawStringLit, *syntax.IntLit:
		return sc.s.recordOf(n) == nil && !sc.s.readsHolder(n), true
	}
	return false, sc.s.recordOf(n) != nil
}

// readsHolder reports a name or selector resolving to a let or a field, which states no key of the
// target but copies the value that let or field holds (API.md R7; DECISIONS 489).
func (s *Snapshot) readsHolder(n syntax.Node) bool {
	var obj check.Object
	switch x := n.(type) {
	case *syntax.IdentExpr:
		obj = s.info.Uses[x]
	case *syntax.SelectorExpr:
		obj = s.info.NameUses[x.Name]
	}
	return obj != nil && (obj.Kind() == check.ObjLet || obj.Kind() == check.ObjField)
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
