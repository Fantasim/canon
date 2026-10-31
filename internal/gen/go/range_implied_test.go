package gogen_test

import (
	"regexp"
	"strings"
	"testing"
)

// local matches a numbered local of the generated decoders.
var local = regexp.MustCompile(`\b[nvx][0-9]+\b`)

// guardOf is the `if !(…) {` line before the line of src that fails with message, its locals named v; "" when none does.
func guardOf(src, message string) string {
	lines := strings.Split(src, "\n")
	for i, line := range lines {
		if i > 0 && strings.Contains(line, message) {
			return local.ReplaceAllString(strings.TrimSpace(lines[i-1]), "v")
		}
	}
	return ""
}

// generatedSource is the one .gen.go file of a generated package.
func generatedSource(files map[string][]byte) string {
	for _, path := range sortedPaths(files) {
		if strings.HasSuffix(path, ".gen.go") {
			return string(files[path])
		}
	}
	return ""
}

// CODEGEN.md §5.15: a side the stored type guarantees writes no comparison, and a Float side always compares; the Float32 is judged in binary64, a Duration in milliseconds, a string by its bytes and a list by its count.
func TestGoRangeImpliedSidesWriteNoCompare(t *testing.T) {
	src := generatedSource(generateData(t, rangePackage()))
	cases := []struct{ message, guard string }{
		{"expected a value in 0..=255", ""},
		{"expected a value in 0..=300", "if !(int64(v) >= 0) {"},
		{"expected a value in ..=0.1", "if !(float64(v) <= 0.1) {"},
		{"expected a value in 0..1", "if !(v >= 0 && v < 1) {"},
		{"expected a value in 1s..=1h", "if !(v.Milliseconds() >= 1000 && v.Milliseconds() <= 3600000) {"},
		{"expected a value in 1s..=2s", "if !(v.Milliseconds() >= 1000 && v.Milliseconds() <= 2000) {"},
		{"expected a byte length in 1..=4", "if !(int64(len(v)) >= 1 && int64(len(v)) <= 4) {"},
		{"expected an element count in ..=3", "if !(int64(v.Len()) <= 3) {"},
	}
	for _, c := range cases {
		got := guardOf(src, c.message)
		if c.guard == "" && strings.Contains(src, c.message) {
			t.Errorf("%s: both sides are implied by the width, but a check is written: %s", c.message, got)
		}
		if got != c.guard {
			t.Errorf("%s: guard %q, want %q", c.message, got, c.guard)
		}
	}
}
