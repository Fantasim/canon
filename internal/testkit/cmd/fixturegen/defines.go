package main

import (
	"fmt"
	"slices"
	"strings"
	"unicode/utf8"
)

// defines keeps a C header's include guard and the #define lines of the listed names, byte
// for byte and in the header's order (tabs and spacing are real shapes the loader must read).
func defines(src []byte, names []string) ([]byte, error) {
	lines := strings.Split(strings.ReplaceAll(string(src), crlf, lineBreak), lineBreak)
	found := map[string]bool{}
	var kept []string
	for _, l := range lines {
		m := reDefine.FindStringSubmatch(l)
		if m == nil || !slices.Contains(names, m[1]) {
			continue
		}
		if found[m[1]] || !utf8.ValidString(l) {
			return nil, fmt.Errorf("%w: %s is defined twice, or its line is not UTF-8", errExtract, m[1])
		}
		found[m[1]] = true
		kept = append(kept, l)
	}
	for _, n := range names {
		if !found[n] {
			return nil, fmt.Errorf("%w: %s is not defined", errExtract, n)
		}
	}
	body := strings.Join(kept, lineBreak) + lineBreak
	if guard := includeGuard(lines); guard != "" {
		body = guardOpen + " " + guard + lineBreak + defineWord + " " + guard + lineBreak + lineBreak +
			body + lineBreak + guardClose + lineBreak
	}
	return []byte(body), nil
}

// includeGuard is the macro of an `#ifndef G` line followed by `#define G`, or "".
func includeGuard(lines []string) string {
	for i := 0; i+1 < len(lines); i++ {
		f := strings.Fields(lines[i])
		if len(f) == guardFields && f[0] == guardOpen {
			next := strings.Fields(lines[i+1])
			if slices.Equal(next, []string{defineWord, f[1]}) {
				return f[1]
			}
			return ""
		}
	}
	return ""
}
