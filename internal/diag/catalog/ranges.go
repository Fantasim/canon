package catalog

import (
	"strconv"
	"strings"
)

// codeRange is one range of a section title: "E10xx" or "E80xx–E83xx" (numbers compared
// on the width of their written digits).
type codeRange struct {
	letter byte
	lo, hi int
	width  int
}

// titleRanges reads the ranges a section title starts with, before its colon.
func titleRanges(title string) []codeRange {
	head, _, _ := strings.Cut(title, titleSep)
	var out []codeRange
	for item := range strings.SplitSeq(head, listSep) {
		lo, hi, found := strings.Cut(item, rangeDash)
		if !found {
			hi = lo
		}
		a, okA := rangeBound(lo)
		b, okB := rangeBound(hi)
		if okA && okB && a.letter == b.letter && a.width == b.width {
			out = append(out, codeRange{letter: a.letter, lo: a.lo, hi: b.lo, width: a.width})
		}
	}
	return out
}

// rangeBound reads "E80xx" as letter E, number 80 of width 2.
func rangeBound(s string) (codeRange, bool) {
	m := reRangeBound.FindStringSubmatch(s)
	if m == nil {
		return codeRange{}, false
	}
	n, err := strconv.Atoi(m[rangeDigitsGroup])
	if err != nil {
		return codeRange{}, false
	}
	return codeRange{letter: m[rangeLetterGroup][0], lo: n, width: len(m[rangeDigitsGroup])}, true
}

// inRanges reports whether the code falls in one of the ranges.
func inRanges(code string, ranges []codeRange) bool {
	for _, r := range ranges {
		if len(code) <= r.width || code[0] != r.letter {
			continue
		}
		n, err := strconv.Atoi(code[1 : 1+r.width])
		if err == nil && n >= r.lo && n <= r.hi {
			return true
		}
	}
	return false
}
