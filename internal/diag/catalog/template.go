package catalog

import (
	"fmt"
	"strings"
)

// placeholders parses a template and lists its placeholders' names in order (ERRORS.md §1.2).
func placeholders(tpl string) ([]string, error) {
	var names []string
	for i := 0; i < len(tpl); {
		name, width, err := templateToken(tpl[i:])
		if err != nil {
			return nil, fmt.Errorf("%w: %q at byte %d: %w", errTemplate, tpl, i, err)
		}
		if name != "" {
			names = append(names, name)
		}
		i += width
	}
	return names, nil
}

// templateToken reads one token at the start of s: an escape, a placeholder (its name is
// returned) or one byte of text. It returns the token's width in bytes.
func templateToken(s string) (string, int, error) {
	if w, ok := escapeWidth(s); ok {
		return "", w, nil
	}
	switch s[0] {
	case openBrace[0]:
		end := strings.IndexByte(s, closeBrace[0])
		if end < 0 || !reLowerCamel.MatchString(s[1:end]) {
			return "", 0, errLoneBrace
		}
		return s[1:end], end + 1, nil
	case closeBrace[0]:
		return "", 0, errLoneBrace
	case escapeChar:
		return "", 0, errEscape
	case backtick[0], cellSep[0]:
		return "", 0, errMarkdown
	}
	return "", 1, nil
}

// escapeWidth reports whether s starts with one of the four escapes of ERRORS.md §1.2.
func escapeWidth(s string) (int, bool) {
	for _, e := range templateEscapes {
		if strings.HasPrefix(s, e) {
			return len(e), true
		}
	}
	return 0, false
}
