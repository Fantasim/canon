package cppgen

import (
	_ "embed"
	"regexp"
	"strings"
)

// stdHeadersText is CODEGEN.md §2.7's table: a standard name's pattern and its header, per line, in header order.
var (
	//go:embed text/std_headers.txt
	stdHeadersText string
)

// stdUse is a standard name's pattern and the header declaring it.
type stdUse struct {
	re     *regexp.Regexp
	header string
}

// stdUses are the table's entries, compiled once.
var stdUses = parseStdUses(stdHeadersText)

func parseStdUses(text string) []stdUse {
	var out []stdUse
	for l := range strings.SplitSeq(strings.TrimSuffix(text, newline), newline) {
		pattern, header, _ := strings.Cut(l, space)
		out = append(out, stdUse{re: regexp.MustCompile(pattern), header: header})
	}
	return out
}

// includes writes the standard headers the text uses, sorted, then each group (CODEGEN.md §2.7).
func includes(out *writer, text string, groups [][]string) {
	std := stdIncludes(text)
	if len(std) > 0 {
		for _, h := range std {
			out.printf(includeFormat, h)
		}
		out.blank()
	}
	for _, grp := range groups {
		for _, h := range grp {
			out.line(h)
		}
		out.blank()
	}
}

// stdIncludes are the standard headers of every standard name the code (comments aside) uses, in byte order.
func stdIncludes(text string) []string {
	var code strings.Builder
	for l := range strings.SplitSeq(text, newline) {
		if !strings.HasPrefix(strings.TrimSpace(l), commentStart) {
			code.WriteString(l)
			code.WriteString(newline)
		}
	}
	var out []string
	for _, u := range stdUses {
		if u.re.MatchString(code.String()) {
			out = append(out, u.header)
		}
	}
	return out
}
