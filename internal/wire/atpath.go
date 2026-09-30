package wire

import (
	"strconv"
	"strings"
)

// AtStep is one step of a parsed `at:` path: a member name, an array index, or `*` (WIRE.md §6.3).
type AtStep struct {
	Kind  AtKind
	Name  string
	Index int
}

// AtKind is what an `at:` step selects.
type AtKind uint8

// ParseAt reads path per WIRE.md §6.3's grammar; false for an empty or malformed path.
func ParseAt(path string) ([]AtStep, bool) {
	if path == "" {
		return nil, false
	}
	step, rest, ok := atFirst(path)
	if !ok {
		return nil, false
	}
	steps := []AtStep{step}
	for rest != "" {
		next, tail, ok := atStepAfter(rest)
		if !ok {
			return nil, false
		}
		steps, rest = append(steps, next), tail
	}
	return steps, true
}

func atFirst(s string) (AtStep, string, bool) {
	switch s[0] {
	case atStarMark:
		return AtStep{Kind: AtStar}, s[1:], true
	case atOpenIndex:
		return atIndexOf(s)
	default:
		return atNameOf(s)
	}
}

// atStepAfter is one "next" step: "." (name|"*"), or "[n]" directly (WIRE.md §6.3).
func atStepAfter(s string) (AtStep, string, bool) {
	switch s[0] {
	case atDot:
		return atDotted(s[1:])
	case atOpenIndex:
		return atIndexOf(s)
	default:
		return AtStep{}, "", false
	}
}

func atDotted(s string) (AtStep, string, bool) {
	if s == "" {
		return AtStep{}, "", false
	}
	if s[0] == atStarMark {
		return AtStep{Kind: AtStar}, s[1:], true
	}
	return atNameOf(s)
}

// atIndexOf reads a leading "[0]" or "[nonzero digits]" (WIRE.md §6.3); s[0] is "[".
func atIndexOf(s string) (AtStep, string, bool) {
	end, ok := atIndexEnd(s)
	if !ok {
		return AtStep{}, "", false
	}
	n, err := strconv.Atoi(s[1:end])
	if err != nil {
		return AtStep{}, "", false
	}
	return AtStep{Kind: AtIndex, Index: n}, s[end+1:], true
}

// atIndexEnd is the index of "]" closing s's leading digits, or false when malformed.
func atIndexEnd(s string) (int, bool) {
	i := 1
	switch {
	case i >= len(s) || !atDigit(s[i]):
		return 0, false
	case s[i] == digitZero:
		i++
	default:
		for i < len(s) && atDigit(s[i]) {
			i++
		}
	}
	if i >= len(s) || s[i] != atCloseIndex {
		return 0, false
	}
	return i, true
}

// atNameOf reads a leading name, `\` escaping one of ". * [ ] \" (WIRE.md §6.3).
func atNameOf(s string) (AtStep, string, bool) {
	var b strings.Builder
	i := 0
	for i < len(s) && !strings.ContainsRune(atStopChars, rune(s[i])) {
		if s[i] != atEscape {
			b.WriteByte(s[i])
			i++
			continue
		}
		if i+1 >= len(s) || !strings.ContainsRune(atEscapable, rune(s[i+1])) {
			return AtStep{}, "", false
		}
		b.WriteByte(s[i+1])
		i += atEscapeLen
	}
	if b.Len() == 0 {
		return AtStep{}, "", false
	}
	return AtStep{Kind: AtName, Name: b.String()}, s[i:], true
}

func atDigit(c byte) bool { return digitZero <= c && c <= digitNine }
