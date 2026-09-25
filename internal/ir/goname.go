package ir

import (
	"go/token"
	"strings"
	"unicode"

	"github.com/fantasim/canonlang/internal/check"
)

// goUpperCamel is Go's UpperCamel(x) (CODEGEN.md §3.2): GoCap of every word.
func goUpperCamel(name string) string {
	var b strings.Builder
	for _, w := range words(name) {
		b.WriteString(goCap(w))
	}
	return b.String()
}

// goLowerCamel is Go's lowerCamel(x) (CODEGEN.md §3.2): the first word lower case, then goUpperCamel of the rest.
func goLowerCamel(name string) string {
	ws := words(name)
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

// goEscapeLower suffixes `_` to a name reserved in a Go lower-case position: a keyword, a predeclared identifier, the name of a package a generated file imports, or `self` (CODEGEN.md §3.4).
func goEscapeLower(name string) string {
	if goReserved(name) {
		return name + underscore
	}
	return name
}

// goStorageName is the unexported, escaped name of a field, parameter or value (CODEGEN.md §3.4, §6.1).
func goStorageName(canon string) string { return goEscapeLower(goLowerCamel(canon)) }

// goEffectiveStore is the storage of a field, method or value: from its @go(name:) override when there is one, so the override renames it whole (decision 203; meta/decisions/log-2026-09-24.md, IR round 2 review).
func goEffectiveStore(n NameOptions, canon string) string {
	return goStorageName(goEffective(n, canon))
}

// goEffective is the name every generated storage name derives from: the @go(name:) override, else the Canon name (decision 203).
func goEffective(n NameOptions, canon string) string {
	if n.Name != "" {
		return n.Name
	}
	return canon
}

// goExported is UpperCamel(canon), or the whole @go(name:) override (CODEGEN.md §3.3, §3.5).
func goExported(n NameOptions, canon string) string {
	if n.Name != "" {
		return n.Name
	}
	return goUpperCamel(canon)
}

// goTypeName is a type's Go name: its first letter upper-cased, or the whole @go(name:) override (CODEGEN.md §3.3).
func goTypeName(n NameOptions, canon string) string {
	if n.Name != "" || canon == "" {
		return n.Name
	}
	return strings.ToUpper(canon[:1]) + canon[1:]
}

// goValidOverride reports a @go(name:) override Go can declare as public API: an exported identifier (CODEGEN.md §1.3, §3.5, decision 182; E8011).
func goValidOverride(name string) bool { return token.IsIdentifier(name) && token.IsExported(name) }

// goValidIdent is the namer's identifier test for the go plan: Go has no `reserved` E8011 variant.
func goValidIdent(name string) (ok, reserved bool) { return token.IsIdentifier(name), false }

// cppValidIdent reports whether name can be declared in C++: a plain identifier that is not a keyword, an alternative token or a name generated code reserves, and not reserved to the implementation, holding `__` or starting with `_` and an upper-case letter (CODEGEN.md §3.4, §3.5; log-2026-09-24 "ir name plans + support plan": E8011). reserved is set when name fails for a reserved reason (E8011 `reserved`) rather than for not being a plain identifier at all (E8011 `derived`).
func cppValidIdent(name string) (ok, reserved bool) {
	if !identPattern.MatchString(name) {
		return false, false
	}
	if CppReserved(name) || strings.Contains(name, cppReservedRun) || cppReservedStart.MatchString(name) {
		return false, true
	}
	return true, false
}

// goReserved reports what a Go lower-case position escapes: a Go keyword, a predeclared identifier, a package a generated file imports, or `self` (CODEGEN.md §3.4).
func goReserved(name string) bool {
	return check.IsGoKeyword(name) || goPredeclared[name] || goImportNames[name] || name == goSelf
}

// CppReserved reports what a C++ verbatim position escapes: a C++20 keyword or alternative token, or a name generated code reserves for itself (CODEGEN.md §3.4).
func CppReserved(name string) bool {
	return check.IsCppKeyword(name) || check.IsCppNamespace(name) || cppOwnNames[name]
}

// words splits a Canon identifier into words, CODEGEN.md §3.1 (every target splits the same way).
func words(name string) []string {
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
