package load

import (
	"strings"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
)

// splitGlob is pattern's literal prefix and its glob rest, split before the first magic character (WIRE.md §6.5).
func splitGlob(pattern string) (prefix, rest string) {
	i := strings.IndexAny(pattern, magicChars)
	if i < 0 {
		return pattern, ""
	}
	seg := strings.LastIndexByte(pattern[:i], '/') + 1
	return pattern[:seg], pattern[seg:]
}

// globBase resolves pattern's literal prefix to a directory, or the file itself with no magic character; a backslash anywhere in pattern is E7001, checked first (WIRE.md §6.5).
func (l *Loader) globBase(pattern, from string, span source.Span, bag *diag.Bag) (project.Path, string, bool) {
	if strings.Contains(pattern, backslash) {
		diag.E7001.AtBackslash(span, pattern).Report(bag)
		return project.Path{}, "", false
	}
	prefix, rest := splitGlob(pattern)
	base, ok := l.Layout.Resolve(prefix, from, span, bag)
	return base, rest, ok
}

// globCheck is validateGlob's verdict: e7001, e7005Cause, dirOnly, or well-formed to walk.
type globCheck struct {
	e7001      bool
	e7005Cause string
	dirOnly    bool
}

// validateGlob checks rest, the glob after its literal base (WIRE.md §2.1, §6.5).
func validateGlob(rest string) globCheck {
	if rest == "" {
		return globCheck{}
	}
	segs, dirOnly := cutTrailingSlash(rest)
	for _, seg := range segs {
		switch seg {
		case "", dotSeg, dotDotSeg:
			return globCheck{e7001: true}
		}
		if cause, ok := validateSegment(seg); !ok {
			return globCheck{e7005Cause: cause}
		}
	}
	return globCheck{dirOnly: dirOnly}
}

// cutTrailingSlash is rest's segments, and whether it ended in exactly one "/" (WIRE.md §2.1).
func cutTrailingSlash(rest string) ([]string, bool) {
	segs := strings.Split(rest, sepStr)
	if last := len(segs) - 1; last >= 0 && segs[last] == "" {
		return segs[:last], true
	}
	return segs, false
}

// validateSegment is one glob segment's E7005 cause: a whole "**", every "[" and "{" closed (WIRE.md §6.5).
func validateSegment(seg string) (string, bool) {
	if strings.Contains(seg, doubleStar) && seg != doubleStar {
		return causeDoubleStar, false
	}
	for i := 0; i < len(seg); i++ {
		var next int
		var ok bool
		switch seg[i] {
		case '[':
			next, ok = skipClass(seg, i)
		case '{':
			next, ok = skipBrace(seg, i)
		default:
			continue
		}
		if !ok {
			return causeOfDelim(seg[i]), false
		}
		i = next - 1
	}
	return "", true
}

// causeOfDelim is E7005's cause for an unclosed delimiter, "[" or "{".
func causeOfDelim(c byte) string {
	if c == '[' {
		return causeBracket
	}
	return causeBrace
}

// skipClass is the index past a "[...]" class at i, or false when unclosed (WIRE.md §6.5).
func skipClass(seg string, i int) (int, bool) {
	j := i + 1
	if j < len(seg) && seg[j] == '!' {
		j++
	}
	if j < len(seg) && seg[j] == ']' {
		j++
	}
	k := strings.IndexByte(seg[j:], ']')
	if k < 0 {
		return 0, false
	}
	return j + k + 1, true
}

// skipBrace is the index past a "{a,b,c}" alternative at i, or false when unclosed, nested or
// with an empty alternative; a class inside is skipped whole, as doublestar skips it.
func skipBrace(seg string, i int) (int, bool) {
	empty := true
	for j := i + 1; j < len(seg); j++ {
		switch seg[j] {
		case '{':
			return 0, false
		case '}':
			return j + 1, !empty
		case ',':
			if empty {
				return 0, false
			}
			empty = true
		case '[':
			end, ok := skipClass(seg, j)
			if !ok {
				return 0, false
			}
			j, empty = end-1, false
		default:
			empty = false
		}
	}
	return 0, false
}
