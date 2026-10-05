package cppgen

import (
	"regexp"
	"slices"

	"github.com/fantasim/canonlang/internal/ir"
)

// WrittenGuard is the error gen/cpp gives a hook of another package's class its owner does not write.
func WrittenGuard(h ir.CppHook) error {
	g := &gen{}
	g.written(h)
	return g.err
}

// PatternData is re's kPattern table as the numbers gen/cpp writes, for the matcher's differential test.
func PatternData(re *regexp.Regexp) ([]int, error) {
	a, err := ir.CompilePattern(re)
	if err != nil {
		return nil, err
	}
	return slices.Concat(patternRows(a)...), nil
}
