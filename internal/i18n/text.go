package i18n

import (
	"strings"
	"unicode"

	"github.com/fantasim/canonlang/internal/syntax"
)

// Kind is a catalogue entry's text kind (I18N.md L5).
type Kind uint8

// literalRuns is the text s contributes outside its interpolations (I18N.md L6): the decoded
// text of a plain string, or the concatenated text parts of an interpolated one.
func literalRuns(s syntax.StrLit) string {
	switch x := s.(type) {
	case *syntax.RawStringLit:
		return x.Value
	case *syntax.StringLit:
		var b strings.Builder
		for _, p := range x.Parts {
			if p.Interp == nil {
				b.WriteString(p.Text)
			}
		}
		return b.String()
	default:
		return ""
	}
}

// hasInterp reports whether s interpolates (I18N.md F6): a raw string never does.
func hasInterp(s syntax.StrLit) bool { return firstInterp(s) != nil }

// firstInterp is s's first interpolation, nil for a raw string or one with none (I18N.md F6:
// E1707 is reported there, not at the key).
func firstInterp(s syntax.StrLit) *syntax.Interp {
	x, ok := s.(*syntax.StringLit)
	if !ok {
		return nil
	}
	for _, p := range x.Parts {
		if p.Interp != nil {
			return p.Interp
		}
	}
	return nil
}

// translatable reports whether text holds at least one Unicode letter (I18N.md L6).
func translatable(text string) bool {
	for _, r := range text {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

// sourceText reconstructs s in the one translation-file form (I18N.md F6): literal braces
// doubled, interpolations as written, whatever s's string kind (plain, raw, multiline).
func sourceText(f *syntax.File, s syntax.StrLit) string {
	switch x := s.(type) {
	case *syntax.RawStringLit:
		var b strings.Builder
		writeEscaped(&b, x.Value)
		return b.String()
	case *syntax.StringLit:
		return stringLitSource(f, x)
	default:
		return ""
	}
}

func stringLitSource(f *syntax.File, x *syntax.StringLit) string {
	var b strings.Builder
	for _, p := range x.Parts {
		if p.Interp == nil {
			writeEscaped(&b, p.Text)
			continue
		}
		sp := f.Span(p.Interp)
		b.WriteByte('{')
		b.Write(f.Src.Content[sp.Start:sp.End])
		b.WriteByte('}')
	}
	return b.String()
}

// writeEscaped doubles a literal '{' or '}' (GRAMMAR.md §2.6, I18N.md F6).
func writeEscaped(b *strings.Builder, text string) {
	for _, r := range text {
		if r == '{' || r == '}' {
			b.WriteRune(r)
		}
		b.WriteRune(r)
	}
}
