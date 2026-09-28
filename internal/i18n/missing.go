package i18n

import (
	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// reportMissing sets each language's Missing count and, for a selected package with an emit
// view, reports one W1701 per language whose count is greater than 0 (I18N.md W1-W3).
func reportMissing(pkg *check.Package, langs []string, r *Result, bag *diag.Bag, emitsView bool) {
	for _, lang := range langs {
		l := r.Languages[lang]
		l.Missing = len(r.Catalogue.Entries) - len(l.Texts)
		if emitsView && l.Missing > 0 {
			reportW1701(pkg, lang, l.Missing, bag)
		}
	}
}

// reportW1701 reports one W1701 for (pkg, lang), the "one" variant when missing is 1 (I18N.md
// W3, ERRORS.md's "one"/"many" convention).
func reportW1701(pkg *check.Package, lang string, missing int, bag *diag.Bag) {
	span := w1701Span(pkg, lang)
	if missing == 1 {
		diag.W1701.AtOne(span, pkg.Path, lang).Report(bag)
		return
	}
	diag.W1701.AtMany(span, int64(missing), pkg.Path, lang).Report(bag)
}

// w1701Span is the location of pkg's W1701 finding for lang (I18N.md W2): the package's first
// translation file for lang, in path order; without one, its first source file (path order).
func w1701Span(pkg *check.Package, lang string) source.Span {
	for _, f := range pkg.Files {
		if f.FileKind == syntax.FileTranslation && f.Lang != nil && f.Lang.Name == lang {
			return wholeLine(f, f.Span(f.Lang))
		}
	}
	for _, f := range pkg.Files {
		if f.FileKind == syntax.FileSource {
			return wholeLine(f, f.Span(f.Package))
		}
	}
	return source.Span{}
}

// wholeLine is the span of sp's whole line, start to end (W2: "the translation line", "the
// package clause"'s line).
func wholeLine(f *syntax.File, sp source.Span) source.Span {
	content := f.Src.Content
	start, end := sp.Start, sp.End
	for start > 0 && content[start-1] != '\n' {
		start--
	}
	for int(end) < len(content) && content[end] != '\n' {
		end++
	}
	return source.Span{File: sp.File, Start: start, End: end}
}
