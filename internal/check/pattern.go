package check

import (
	"math"
	"regexp"
	"strings"
	"unicode/utf8"
)

// patternScan whitelists a pattern's spelling, which RE2's tree loses (EVALUATION.md §11.3).
type patternScan struct {
	re     string
	i      int
	anchor string // the anchor just read: ECMAScript quantifies no bare anchor
	esc    string // the class escape just read in a class: ECMAScript starts no range from it
}

// patternStep reads the construct at s.i and advances past it; it returns the construct's
// spelling when it is outside the subset (and may then stop anywhere).
type patternStep func(*patternScan) string

// outsideClass and insideClass dispatch on a byte; a nil entry is a literal character.
var outsideClass, insideClass [math.MaxUint8 + 1]patternStep

var repeatRe = regexp.MustCompile(repeatPattern)

func init() {
	outsideClass[backslash], outsideClass['('], outsideClass['['] = (*patternScan).escape, (*patternScan).group, (*patternScan).class
	outsideClass['{'], outsideClass['}'], outsideClass[']'] = (*patternScan).brace, (*patternScan).stray, (*patternScan).stray
	outsideClass['^'], outsideClass['$'] = (*patternScan).anchorAt, (*patternScan).anchorAt
	outsideClass['*'], outsideClass['+'], outsideClass['?'] = (*patternScan).quantifier, (*patternScan).quantifier, (*patternScan).quantifier
	insideClass[backslash], insideClass['['], insideClass['-'] = (*patternScan).classEscape, (*patternScan).nested, (*patternScan).dash
}

// unportable is the first construct of re outside the portable subset, "" when there is none;
// re compiled as RE2 (E1114 otherwise), so every escape and class is complete.
func unportable(re string) string {
	s := &patternScan{re: re}
	for s.i < len(re) {
		step := outsideClass[re[s.i]]
		if step == nil {
			step = (*patternScan).literal
		}
		if bad := step(s); bad != "" {
			return bad
		}
	}
	return ""
}

// peek is the byte k past s.i, 0 past the end.
func (s *patternScan) peek(k int) byte {
	if s.i+k < len(s.re) {
		return s.re[s.i+k]
	}
	return 0
}

// spelledAt is the text from s.i through the rune starting k bytes past it.
func (s *patternScan) spelledAt(k int) string {
	_, n := utf8.DecodeRuneInString(s.re[s.i+k:])
	return s.re[s.i : s.i+k+n]
}

func (s *patternScan) literal() string {
	s.anchor = ""
	s.i++
	return ""
}

// escaped is `\` and one rune, outside the subset unless allowed lists it.
func (s *patternScan) escaped(allowed string) string {
	spelled := s.spelledAt(1)
	s.i += len(spelled)
	r, _ := utf8.DecodeRuneInString(spelled[1:])
	if !strings.ContainsRune(allowed, r) {
		return spelled
	}
	return ""
}

func (s *patternScan) escape() string {
	s.anchor = ""
	return s.escaped(patternEscapes)
}

// group is `(` or `(?:`; every other `(?` is a flag or a named group.
func (s *patternScan) group() string {
	s.anchor = ""
	switch {
	case s.peek(1) != '?':
		s.i++
	case s.peek(len(flagGroup)) == ':':
		s.i += len(plainGroup)
	default:
		return s.spelledAt(len(flagGroup))
	}
	return ""
}

// brace is a `{n}`, `{n,}` or `{n,m}` quantifier; RE2 reads any other `{` as a literal.
func (s *patternScan) brace() string {
	m := repeatRe.FindString(s.re[s.i:])
	if m == "" {
		return s.spelledAt(0)
	}
	return s.quantified(len(m))
}

// stray is a `}` or `]` RE2 reads as a literal and ECMAScript refuses.
func (s *patternScan) stray() string { return s.spelledAt(0) }

func (s *patternScan) anchorAt() string {
	s.anchor = s.spelledAt(0)
	s.i++
	return ""
}

func (s *patternScan) quantifier() string { return s.quantified(1) }

// quantified is a quantifier n bytes long, outside the subset right after an anchor.
func (s *patternScan) quantified(n int) string {
	spelled := s.re[s.i : s.i+n]
	s.i += n
	if s.anchor != "" {
		return s.anchor + spelled
	}
	return ""
}

// class is a bracket class, `[…]` or `[^…]`; a `]` first is RE2's literal, which ECMAScript
// reads as an empty class.
func (s *patternScan) class() string {
	start := s.i
	s.i++
	if s.peek(0) == '^' {
		s.i++
	}
	if s.peek(0) == ']' {
		return s.re[start : s.i+1]
	}
	s.esc = ""
	for s.i < len(s.re) && s.re[s.i] != ']' {
		step := insideClass[s.re[s.i]]
		if step == nil {
			step = (*patternScan).classLiteral
		}
		if bad := step(s); bad != "" {
			return bad
		}
	}
	s.i++
	s.anchor = ""
	return ""
}

func (s *patternScan) classLiteral() string {
	s.esc = ""
	s.i++
	return ""
}

func (s *patternScan) classEscape() string {
	s.esc = ""
	if strings.IndexByte(perlClassLetters, s.peek(1)) >= 0 {
		s.esc = s.spelledAt(1)
	}
	return s.escaped(classEscapes)
}

// nested is a `[` inside a class: a POSIX class `[:…:]`, or a literal only RE2 and ECMAScript's
// legacy modes agree on.
func (s *patternScan) nested() string { return s.spelledAt(boolCount(s.peek(1) == ':')) }

// dash is a `-` in a class: a range, or a literal first or last; never a range from a class escape.
func (s *patternScan) dash() string {
	esc := s.esc
	s.esc = ""
	s.i++
	if esc != "" && s.peek(0) != ']' {
		return esc + hyphen
	}
	return ""
}
