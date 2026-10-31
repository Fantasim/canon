package cppruntime

import (
	_ "embed"
	"maps"
	"slices"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

var (
	//go:embed canon_runtime.h.txt
	Header string
	//go:embed canon_runtime_json.h.txt
	JSON string
)

// Identifiers are the identifier tokens of Header and JSON outside comments, string and character literals, sorted and distinct: the names a macro of a translation unit that includes them would rewrite (CODEGEN.md §7.9).
var Identifiers = sync.OnceValue(func() []string {
	found := map[string]bool{}
	add := func(word string) { found[word] = true }
	Scan(Header, add)
	Scan(JSON, add)
	return slices.Sorted(maps.Keys(found))
})

// Scan calls visit with each identifier token of C++ text outside comments, string and character literals, and numbers; a `//` comment ends at its first newline, a backslash before it included (CODEGEN.md §7.9, DECISIONS 404).
func Scan(text string, visit func(word string)) {
	for i := 0; i < len(text); {
		rest := text[i:]
		switch n := commentLen(rest); {
		case n > 0:
			i += n
		case text[i] == quoteMark || text[i] == charMark:
			i += skipQuoted(rest)
		case wordByte(text[i]):
			end := i + wordLen(rest)
			if !unicode.IsDigit(rune(text[i])) {
				visit(text[i:end])
			}
			i = end
		default:
			i++
		}
	}
}

// Code is C++ text with each comment replaced by one blank and its string and character literals kept whole: the text a standard name is looked for in (CODEGEN.md §2.7, DECISIONS 423 pending).
func Code(text string) string {
	var b strings.Builder
	for i := 0; i < len(text); {
		rest := text[i:]
		n := commentLen(rest)
		switch {
		case n > 0:
			b.WriteString(blank)
		case text[i] == quoteMark || text[i] == charMark:
			n = skipQuoted(rest)
			b.WriteString(rest[:n])
		default:
			n = 1
			b.WriteByte(text[i])
		}
		i += n
	}
	return b.String()
}

// commentLen is the length of the comment that starts text (a `//` one to its first newline, a backslash before it included), 0 when text starts none.
func commentLen(text string) int {
	switch {
	case strings.HasPrefix(text, lineComment):
		return skipTo(text, newline)
	case strings.HasPrefix(text, blockOpen):
		return skipTo(text[len(blockOpen):], blockClose) + len(blockOpen)
	}
	return 0
}

// skipTo is the length of text up to and with the first end, or all of it.
func skipTo(text, end string) int {
	if n := strings.Index(text, end); n >= 0 {
		return n + len(end)
	}
	return len(text)
}

// skipQuoted is the length of the literal that starts text, escapes honoured.
func skipQuoted(text string) int {
	for i := 1; i < len(text); i++ {
		switch text[i] {
		case escapeMark:
			i++
		case text[0]:
			return i + 1
		}
	}
	return len(text)
}

// wordLen is the length of the run of word bytes that starts text.
func wordLen(text string) int {
	n := 0
	for n < len(text) && wordByte(text[n]) {
		n++
	}
	return n
}

// wordByte reports an ASCII letter, digit or underscore: a byte of an identifier or a number.
func wordByte(b byte) bool {
	return b < utf8.RuneSelf && (b == underscore || unicode.IsLetter(rune(b)) || unicode.IsDigit(rune(b)))
}
