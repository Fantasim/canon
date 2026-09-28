package cppgen

import (
	"regexp"
	"slices"

	"github.com/fantasim/canonlang/internal/ir"
)

// PatternData is re's kPattern table as the numbers gen/cpp writes, for the matcher's differential test.
func PatternData(re *regexp.Regexp) ([]int, error) {
	a, err := ir.CompilePattern(re)
	if err != nil {
		return nil, err
	}
	return slices.Concat(patternRows(a)...), nil
}
