package check

import (
	"math"
	"slices"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// syntaxErrors breaks each declaration holding a lexer or parser error from its creation
// (newObject), before anything is folded or evaluated.
func (c *checker) syntaxErrors() {
	spans := c.syntaxSpans()
	for _, p := range c.sorted {
		for _, f := range p.files {
			if errs := spans[f.Src.ID]; len(errs) > 0 {
				c.holdErrors(f, errs)
			}
		}
	}
}

// syntaxSpans are the spans of the syntax errors of the packages' parse findings, by file; the
// lexer's errors inside a literal also go to c.literalErrs.
func (c *checker) syntaxSpans() map[source.FileID][]source.Span {
	codes := map[diag.Code]bool{}
	for _, d := range diag.Registry {
		codes[d.Code] = d.Package == syntaxOwner && d.Severity == diag.Error
	}
	inLiteral := map[diag.Code]bool{}
	for _, d := range literalCodes {
		inLiteral[d.Def().Code] = true
	}
	spans := map[source.FileID][]source.Span{}
	for _, p := range c.sorted {
		for _, f := range parseFindings(p) {
			if codes[f.Code] {
				spans[f.Span.File] = append(spans[f.Span.File], f.Span)
			}
			if inLiteral[f.Code] {
				c.literalErrs[f.Span.File] = append(c.literalErrs[f.Span.File], f.Span)
			}
		}
	}
	return spans
}

// lexError reports a literal holding a lexer error: it has the error type, and its declaration
// is broken already (DECISIONS 215).
func (c *checker) lexError(env *env, e syntax.Expr) bool {
	at := env.span(e)
	return slices.ContainsFunc(c.literalErrs[at.File], func(s source.Span) bool { return encloses(at, s) })
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
		at := f.Span(d)
		for _, e := range errs {
			if encloses(at, e) {
				c.syntaxHeld[innermost(f, d, e)] = true
			}
		}
	}
}

// innermost is the narrowest of d and its member methods and checks that encloses e.
func innermost(f *syntax.File, d syntax.Decl, e source.Span) syntax.Node {
	var best syntax.Node = d
	syntax.Inspect(d, func(n syntax.Node) bool {
		switch n.(type) {
		case *syntax.FnDecl, *syntax.CheckDecl:
			if n != d && encloses(f.Span(n), e) {
				best = n
			}
		}
		return true
	})
	return best
}

// encloses reports a span d that holds the start of the finding at e.
func encloses(d, e source.Span) bool {
	return d.File == e.File && d.Start <= e.Start && e.Start < d.End
}

// leafLiteral reports a literal token, the only node a lexer error gives the error type (DECISIONS 215).
func leafLiteral(e syntax.Expr) bool {
	switch e.(type) {
	case *syntax.IntLit, *syntax.FloatLit, *syntax.DurationLit, *syntax.StringLit, *syntax.RawStringLit, *syntax.RegexLit:
		return true
	}
	return false
}
