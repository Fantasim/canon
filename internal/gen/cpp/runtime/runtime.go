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
	scan(Header, found)
	scan(JSON, found)
	return slices.Sorted(maps.Keys(found))
})

// scan adds the identifier tokens of C++ text to found.
func scan(text string, found map[string]bool) {
	for i := 0; i < len(text); {
		rest := text[i:]
		switch {
		case strings.HasPrefix(rest, lineComment):
			i += skipTo(rest, newline)
		case strings.HasPrefix(rest, blockOpen):
			i += skipTo(rest[len(blockOpen):], blockClose) + len(blockOpen)
		case text[i] == quoteMark || text[i] == charMark:
			i += skipQuoted(rest)
		case wordByte(text[i]):
			end := i + wordLen(rest)
			if !unicode.IsDigit(rune(text[i])) {
				found[text[i:end]] = true
			}
			i = end
		default:
			i++
		}
	}
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
