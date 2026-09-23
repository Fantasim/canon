package finding

import (
	"strings"
	"unicode/utf8"
)

// Finding is one departure from one rule. Line is display only: the ratchet keys on
// Rule, File, Symbol and Detail, so edits above a finding never make it "new".
type Finding struct {
	Rule    string
	File    string
	Line    int
	Symbol  string
	Detail  string
	Value   int
	Message string
	Fix     string
	// Src is the source file Line refers to when File is a directory (package-scoped).
	Src string
}

func (f Finding) Key() string {
	return strings.Join([]string{f.Rule, f.File, f.Symbol, f.Detail}, keySep)
}

// Normalize turns a tool message into a stable detail: numbers masked, whitespace folded.
func Normalize(msg string) string {
	s := digits.ReplaceAllString(msg, "#")
	s = strings.TrimSpace(spaces.ReplaceAllString(s, " "))
	return clip(s, maxDetail)
}

// clip truncates s to at most n runes, never inside one.
func clip(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}

// Clean makes a value safe for one TSV cell.
func Clean(s string) string {
	return strings.TrimSpace(spaces.ReplaceAllString(s, " "))
}
