package format

import (
	"reflect"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/syntax"
)

// hosts marks the first token of every node that carries a doc block (GRAMMAR.md §9.1).
func hosts(f *syntax.File) []bool {
	out := make([]bool, len(f.Tokens))
	syntax.Inspect(f, func(n syntax.Node) bool {
		if n != nil && documented(n) {
			out[n.First()] = true
		}
		return true
	})
	return out
}

// documented reports a node whose type has a Doc field.
func documented(n syntax.Node) bool {
	t := reflect.TypeOf(n)
	if t.Kind() != reflect.Pointer {
		return false
	}
	_, ok := t.Elem().FieldByName(docField)
	return ok
}

// keptCommas marks the commas DECISIONS 216 keeps.
func keptCommas(f *syntax.File) []bool {
	host, sorted := hosts(f), make([]bool, len(f.Tokens))
	for _, imp := range f.Imports {
		if imp.Braces.Open != syntax.NoTok {
			for t := imp.Braces.Open; t < imp.Braces.Close; t++ {
				sorted[t] = true
			}
		}
	}
	out := make([]bool, len(f.Tokens))
	for i := range f.Tokens {
		out[i] = guards(f, i, host, sorted[i])
	}
	return out
}

// guards reports the comma i that DECISIONS 216 keeps; one between sorted import names is kept
// whatever follows it.
func guards(f *syntax.File, i int, host []bool, sorted bool) bool {
	t := f.Tokens[i]
	if t.Kind != syntax.TokComma || !startsLine(t) {
		return false
	}
	lead, blank := leading(f, t.Leading)
	trail, _ := trailing(f, t.Trailing)
	if slices.ContainsFunc(dropped(lead, trail, true)[len(lead):], becomesDoc) {
		return true
	}
	return openDoc(lead) && !blank && (sorted || reaches(f, following(f, i), host))
}

// becomesDoc reports a "///" comment written after code that would start its own line.
func becomesDoc(n note) bool {
	return !n.sameLine && strings.HasPrefix(n.text, docLead) && !strings.HasPrefix(n.text, ordinaryLead)
}

// openDoc reports notes ending in a doc block no blank line closes.
func openDoc(notes []note) bool {
	open := false
	for _, n := range notes {
		open = n.doc || open && !n.blank
	}
	return open
}

// reaches reports whether a doc block moved right before token j merges with one there or
// attaches to j.
func reaches(f *syntax.File, j int, host []bool) bool {
	lead, blank := leading(f, f.Tokens[j].Leading)
	for _, n := range lead {
		if n.blank {
			return false
		}
		if n.doc {
			return true
		}
	}
	return host[j] && !blank
}

// following is the first token after i that is not a separator NL.
func following(f *syntax.File, i int) int {
	j := i + 1
	for j < len(f.Tokens)-1 && f.Tokens[j].Kind == syntax.TokNL {
		j++
	}
	return j
}

// keeps reports a kept comma right after token t.
func (b *builder) keeps(t syntax.Tok) bool {
	c := b.after(t)
	return b.f.Tokens[c].Kind == syntax.TokComma && !b.drop[c]
}

// kept is the kept comma after t on a line of its own, with its comments.
func (b *builder) kept(t syntax.Tok, blanks bool) *doc {
	if !b.keeps(t) {
		return nil
	}
	c := b.after(t)
	var gap *doc
	if blanks && b.gap(c) {
		gap = blankDoc
	}
	return cat(hardlineDoc, gap, b.lead(c, blanks), text(syntax.TokComma.String()), b.trail(c))
}
