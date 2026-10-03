package tsgen

import (
	"regexp"
	"slices"
	"strings"
	"unicode"

	"github.com/fantasim/canonlang/internal/ir"
)

// words splits a Canon identifier into words (CODEGEN.md §3.1).
func words(name string) []string {
	var out []string
	for piece := range strings.SplitSeq(name, underscore) {
		start := 0
		for i := 1; i < len(piece); i++ {
			if boundary(piece, i) {
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

// boundary reports a word boundary between s[i-1] and s[i] (CODEGEN.md §3.1 rule 2).
func boundary(s string, i int) bool {
	a, b := rune(s[i-1]), rune(s[i])
	switch {
	case unicode.IsLower(a) && unicode.IsUpper(b):
		return true
	case unicode.IsLetter(a) && unicode.IsDigit(b), unicode.IsDigit(a) && unicode.IsLetter(b):
		return true
	}
	return unicode.IsUpper(a) && unicode.IsUpper(b) && i+1 < len(s) && unicode.IsLower(rune(s[i+1]))
}

// capWord is Cap(w): the first character upper case, the rest lower case (CODEGEN.md §3.2).
func capWord(w string) string {
	if w == "" {
		return w
	}
	return strings.ToUpper(w[:1]) + strings.ToLower(w[1:])
}

// upperCamel is UpperCamel(x) in C++ and TS casing (CODEGEN.md §3.2).
func upperCamel(name string) string {
	var b strings.Builder
	for _, w := range words(name) {
		b.WriteString(capWord(w))
	}
	return b.String()
}

// lowerCamel is lowerCamel(x): the first word lower case, then UpperCamel of the rest.
func lowerCamel(name string) string {
	ws := words(name)
	if len(ws) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(strings.ToLower(ws[0]))
	for _, w := range ws[1:] {
		b.WriteString(capWord(w))
	}
	return b.String()
}

// upperSnake is the words of name upper case, joined by `_`: the name of a lookup table.
func upperSnake(name string) string {
	return strings.ToUpper(strings.Join(words(name), underscore))
}

// effective is the whole @ts(name:) override, else name (CODEGEN.md §3.5).
func effective(n ir.NameOptions, name string) string {
	if n.Name != "" {
		return n.Name
	}
	return name
}

// typeName is a record's, enum's, variant's or dependent type's TypeScript name: its first letter upper-cased, or the override (CODEGEN.md §3.3).
func typeName(t ir.Type) string {
	name, over := declared(t)
	if over.Name != "" {
		return over.Name
	}
	if name == "" {
		return name
	}
	return strings.ToUpper(name[:1]) + name[1:]
}

// declared is a named type's Canon name and its @ts override.
func declared(t ir.Type) (string, ir.NameOptions) {
	switch x := t.(type) {
	case *ir.Record:
		return x.Name, x.TS
	case *ir.Enum:
		return x.Name, x.TS
	case *ir.Variant:
		return x.Name, x.TS
	case *ir.Dependent:
		return x.Name, x.TS
	}
	return "", ir.NameOptions{}
}

// pkgOf is the Canon package declaring a named type.
func pkgOf(t ir.Type) string {
	switch x := t.(type) {
	case *ir.Record:
		return x.Pkg
	case *ir.Enum:
		return x.Pkg
	case *ir.Variant:
		return x.Pkg
	case *ir.Dependent:
		return x.Pkg
	}
	return ""
}

// kindName is a variant's kind enum, TKind (CODEGEN.md §3.3).
func kindName(v *ir.Variant) string { return typeName(v) + kindSuffix }

// branchName is a dependent type's branch enum, TBranch.
func branchName(d *ir.Dependent) string { return typeName(d) + branchSuffix }

// caseName is a case's type, T + UpperCamel(c), or its override.
func caseName(v *ir.Variant, c *ir.Case) string {
	return effective(c.TS, typeName(v)+upperCamel(c.Name))
}

// idName is the id type of the table of elem: <Element>Id (CODEGEN.md §3.3).
func idName(elem ir.Type) string { return typeName(elem) + idSuffix }

// reserved reports a name a top-level binding or parameter may not take (CODEGEN.md §3.4).
func reserved(name string) bool { return slices.Contains(reservedWords, name) }

// escape suffixes `_` to a reserved name.
func escape(name string) string {
	if reserved(name) {
		return name + underscore
	}
	return name
}

var identifier = regexp.MustCompile(identPattern)

// property is a name as an object key: bare when it is an identifier, else a quoted string.
func property(name string) string {
	if identifier.MatchString(name) {
		return name
	}
	return quote(name)
}
