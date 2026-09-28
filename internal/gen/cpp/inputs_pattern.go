package cppgen

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/fantasim/canonlang/internal/ir"
)

// patternTable declares kPattern, one line per patternRows row (CODEGEN.md §5.12, §7.7).
func (g *gen) patternTable(re *regexp.Regexp) []string {
	a, err := ir.CompilePattern(re)
	if err != nil {
		g.malformed(patternOutside, g.at)
		return nil
	}
	lines := []string{fmt.Sprintf(tableOpenFormat, cppUint32, patternVar)}
	for _, row := range patternRows(a) {
		lines = append(lines, tableRow(row))
	}
	return append(lines, closeClass)
}

// patternRows is a in MatchPattern's layout: the state count, a {op, out, arg, count} row per state, then each distinct set of code point pairs once, in first use order.
func patternRows(a ir.PatternAutomaton) [][]int {
	rows := [][]int{{len(a.States)}}
	next := 1 + patternRow*len(a.States)
	var pairs [][]int
	offsets := map[string]int{}
	for _, s := range a.States {
		arg, count := 0, 0
		switch {
		case s.Op == ir.PatternSplit:
			arg = s.Alt
		case s.Op == ir.PatternAnchor:
			arg = int(s.At)
		case len(s.Runes) > 0:
			set := make([]int, len(s.Runes))
			for i, r := range s.Runes {
				set[i] = int(r)
			}
			key := tableRow(set)
			if _, ok := offsets[key]; !ok {
				offsets[key] = next
				next += len(set)
				pairs = append(pairs, set)
			}
			arg, count = offsets[key], len(set)/runePair
		}
		rows = append(rows, []int{int(s.Op), s.Out, arg, count})
	}
	return append(rows, pairs...)
}

// tableRow is one indented row of a constant table, every number followed by a comma.
func tableRow(ns []int) string {
	items := make([]string, len(ns))
	for i, n := range ns {
		items[i] = strconv.Itoa(n)
	}
	return indentUnit + strings.Join(items, listSep) + comma
}
