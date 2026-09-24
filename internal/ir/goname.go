package ir

import (
	"go/token"
	"strings"
	"unicode"
)

// GoUpperCamel is Go's UpperCamel(x) (CODEGEN.md §3.2), gen/go's own copy so the two cannot drift (decision 120).
func GoUpperCamel(name string) string {
	var b strings.Builder
	for _, w := range GoWords(name) {
		b.WriteString(goCap(w))
	}
	return b.String()
}

// GoLowerCamel is Go's lowerCamel(x) (CODEGEN.md §3.2): the first word lower case, then GoUpperCamel of the rest.
func GoLowerCamel(name string) string {
	ws := GoWords(name)
	if len(ws) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(strings.ToLower(ws[0]))
	for _, w := range ws[1:] {
		b.WriteString(goCap(w))
	}
	return b.String()
}

// goCap is GoCap(w) of CODEGEN.md §3.2: an initialism of the closed list is all upper case, else Cap(w) (the first character upper case, the rest lower case).
func goCap(w string) string {
	if len(w) == 0 {
		return w
	}
	if goInitialisms[strings.ToLower(w)] {
		return strings.ToUpper(w)
	}
	return strings.ToUpper(w[:1]) + strings.ToLower(w[1:])
}

// GoEscapeLower suffixes `_` to a name reserved in a Go lower-case position: a keyword, a predeclared identifier, the name of a package a generated file imports, or `self` (CODEGEN.md §3.4).
func GoEscapeLower(name string) string {
	if goReservedLower[name] {
		return name + underscore
	}
	return name
}

// goValidIdent reports whether name can be declared in Go: an identifier that is not a keyword (CODEGEN.md §3.5, E8011).
func goValidIdent(name string) bool { return token.IsIdentifier(name) }

// cppValidIdent reports whether name can be declared in C++: a plain identifier that is not a keyword, an alternative token or a name generated code reserves (CODEGEN.md §3.5, E8011).
func cppValidIdent(name string) bool { return identPattern.MatchString(name) && !cppReserved[name] }

// GoWords splits a Canon identifier into words, as Go generation does (CODEGEN.md §3.1).
func GoWords(name string) []string {
	var out []string
	for piece := range strings.SplitSeq(name, underscore) {
		start := 0
		for i := 1; i < len(piece); i++ {
			if goWordBoundary(piece, i) {
				out = append(out, piece[start:i])
				start = i
			}
		}
		if start < len(piece) {
			out = append(out, piece[start:])
		}
	}
	return out
}

// goWordBoundary reports a word boundary between s[i-1] and s[i] (CODEGEN.md §3.1 rule 2).
func goWordBoundary(s string, i int) bool {
	a, b := rune(s[i-1]), rune(s[i])
	switch {
	case unicode.IsLower(a) && unicode.IsUpper(b):
		return true
	case unicode.IsLetter(a) && unicode.IsDigit(b), unicode.IsDigit(a) && unicode.IsLetter(b):
		return true
	}
	return unicode.IsUpper(a) && unicode.IsUpper(b) && i+1 < len(s) && unicode.IsLower(rune(s[i+1]))
}
