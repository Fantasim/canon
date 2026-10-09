package check

import (
	"cmp"
	"math"
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// literalCodes are the lexer's findings inside a literal token (DECISIONS 215).
var literalCodes = []interface{ Def() *diag.Def }{
	diag.E1101, diag.E1102, diag.E1107, diag.E1109, diag.E1110, diag.E1111, diag.E1112, diag.E1113, diag.E1114, diag.E1122, diag.E1124,
}

// syntaxErrors breaks each declaration holding a lexer or parser error from its creation
// (newObject), before anything is folded or evaluated, and marks each literal token holding a
// lexer error.
func (c *checker) syntaxErrors() {
	found := c.syntaxSpans(c.sorted)
	for _, p := range c.sorted {
		for _, f := range p.files {
			c.syntaxErrorsIn(f, found)
		}
	}
}

// syntaxErrorsIn is syntaxErrors for one file.
func (c *checker) syntaxErrorsIn(f *syntax.File, found syntaxFound) {
	if errs := byStart(found.all[f.Src.ID]); len(errs) > 0 {
		c.holdErrors(f, errs)
		c.holdTranslationErrors(f, errs)
	}
	if errs := byStart(found.inLiterals[f.Src.ID]); len(errs) > 0 {
		c.markBadLiterals(f, errs)
	}
	if errs := byStart(found.dataWords[f.Src.ID]); len(errs) > 0 {
		c.markUnmatchable(f, errs)
	}
}

// syntaxFound are the spans of the packages' parse findings, by file: every syntax error, the
// lexer's errors inside a literal, and E1126's data words.
type syntaxFound struct {
	all, inLiterals, dataWords map[source.FileID][]source.Span
}

// syntaxSpans sorts the parse findings of pkgs into a syntaxFound.
func (c *checker) syntaxSpans(pkgs []*pkgState) syntaxFound {
	codes := map[diag.Code]bool{}
	for _, d := range diag.Registry {
		codes[d.Code] = d.Package == syntaxOwner && d.Severity == diag.Error
	}
	inLiteral := map[diag.Code]bool{}
	for _, d := range literalCodes {
		inLiteral[d.Def().Code] = true
	}
	found := syntaxFound{all: map[source.FileID][]source.Span{}, inLiterals: map[source.FileID][]source.Span{}, dataWords: map[source.FileID][]source.Span{}}
	for _, p := range pkgs {
		for _, f := range parseFindings(p) {
			if codes[f.Code] {
				found.all[f.Span.File] = append(found.all[f.Span.File], f.Span)
			}
			if inLiteral[f.Code] {
				found.inLiterals[f.Span.File] = append(found.inLiterals[f.Span.File], f.Span)
			}
			if f.Code == diag.E1126.Def().Code {
				found.dataWords[f.Span.File] = append(found.dataWords[f.Span.File], f.Span)
			}
		}
	}
	return found
}

// byStart is a copy of spans sorted by start, the order within reads: a node finds its errors by
// bisection, so a file with N findings costs N log N, not N times its node count.
func byStart(spans []source.Span) []source.Span {
	out := slices.Clone(spans)
	slices.SortFunc(out, func(a, b source.Span) int { return cmp.Compare(a.Start, b.Start) })
	return out
}

// within is the errs, sorted by start, that d encloses (see encloses).
func within(errs []source.Span, d source.Span) []source.Span {
	from := func(s source.Span, p source.Pos) int { return cmp.Compare(s.Start, p) }
	lo, _ := slices.BinarySearchFunc(errs, d.Start, from)
	hi, _ := slices.BinarySearchFunc(errs, d.End, from)
	return errs[lo:max(lo, hi)]
}

// markUnmatchable marks each enum member and variant case whose own name is one of errs (E1126):
// it exists, but no pattern can name it, so no match is asked to cover it.
func (c *checker) markUnmatchable(f *syntax.File, errs []source.Span) {
	syntax.Inspect(f, func(n syntax.Node) bool {
		var name *syntax.Ident
		switch x := n.(type) {
		case *syntax.EnumMember:
			name = x.Name
		case *syntax.VariantCase:
			name = x.Name
		default:
			return true
		}
		if len(within(errs, f.Span(name))) > 0 {
			c.unmatchable[n] = true
		}
		return true
	})
}

// markBadLiterals marks each literal token of f that holds one of errs, wherever it stands: an
// expression, an enum member's value, an annotation argument (DECISIONS 215).
func (c *checker) markBadLiterals(f *syntax.File, errs []source.Span) {
	syntax.Inspect(f, func(n syntax.Node) bool {
		if !leafLiteral(n) {
			return true
		}
		if len(within(errs, f.Span(n))) > 0 {
			c.badLits[n] = true
		}
		return true
	})
}

// lexError reports a literal holding a lexer error: it has the error type, no fold and no static
// check on the value the lexer made up, and its declaration is broken already (DECISIONS 215).
func (c *checker) lexError(n syntax.Node) bool {
	return c.badLits[n]
}

// holdsSyntaxError reports a syntax or lexical error inside d, truncation-proof (syntaxHeld, badLits).
func (c *checker) holdsSyntaxError(d *syntax.ViewDecl) bool {
	if c.syntaxHeld[d] {
		return true
	}
	found := false
	syntax.Inspect(d, func(n syntax.Node) bool {
		if leafLiteral(n) && c.badLits[n] {
			found = true
		}
		return !found
	})
	return found
}

// parseFindings are the parse findings a build put in p's bag, or p's files parsed again when
// the bag dropped some (API.md F7).
func parseFindings(p *pkgState) []diag.Finding {
	if len(p.bag.Summary().Truncated) == 0 {
		return p.bag.Findings()
	}
	bag := diag.NewBag(indexFiles(p.files), p.path)
	bag.Truncate(math.MaxInt)
	for _, f := range p.files {
		syntax.Parse(f.Src, syntax.FileSource, bag)
	}
	return bag.Findings()
}

// holdErrors marks the holder of each error of f: its layer, else the innermost of a declaration
// and its methods and checks, so a method breaks only its callers (DECISIONS 209, 214).
func (c *checker) holdErrors(f *syntax.File, errs []source.Span) {
	if f.Layer != nil {
		c.syntaxHeld[f.Layer] = true
		return
	}
	for _, d := range f.Decls {
		in := within(errs, f.Span(d))
		if len(in) == 0 {
			continue
		}
		members := memberSpans(f, d)
		for _, e := range in {
			c.syntaxHeld[innermost(d, members, e)] = true
		}
	}
}

// holdTranslationErrors marks each translation entry of f holding one of errs (ADR-0009).
func (c *checker) holdTranslationErrors(f *syntax.File, errs []source.Span) {
	for _, e := range f.Entries {
		if len(within(errs, f.Span(e))) > 0 {
			c.info.BrokenTranslations[e] = true
		}
	}
}

// member is a method or check inside a declaration, with its span.
type member struct {
	node syntax.Node
	span source.Span
}

// memberSpans are the methods and checks of d, d itself excluded, in pre-order: read once per
// declaration holding an error, not once per error.
func memberSpans(f *syntax.File, d syntax.Decl) []member {
	var out []member
	syntax.Inspect(d, func(n syntax.Node) bool {
		switch n.(type) {
		case *syntax.FnDecl, *syntax.CheckDecl:
			if n != d {
				out = append(out, member{node: n, span: f.Span(n)})
			}
		}
		return true
	})
	return out
}

// innermost is the narrowest of d and its member methods and checks that encloses e: the last
// of members, in pre-order, that does.
func innermost(d syntax.Decl, members []member, e source.Span) syntax.Node {
	for _, m := range slices.Backward(members) {
		if encloses(m.span, e) {
			return m.node
		}
	}
	return d
}

// encloses reports a span d that holds the start of the finding at e.
func encloses(d, e source.Span) bool {
	return d.File == e.File && d.Start <= e.Start && e.Start < d.End
}

// leafLiteral reports a literal token, the only node a lexer error gives the error type (DECISIONS 215).
func leafLiteral(n syntax.Node) bool {
	switch n.(type) {
	case *syntax.IntLit, *syntax.FloatLit, *syntax.DurationLit, *syntax.StringLit, *syntax.RawStringLit, *syntax.RegexLit:
		return true
	}
	return false
}
