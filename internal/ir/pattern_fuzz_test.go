package ir_test

import (
	"errors"
	"regexp"
	resyntax "regexp/syntax"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/fantasim/canonlang/internal/ir"
)

// FuzzCppPattern is log 2026-09-24 "Pattern semantics" on any pattern Go compiles: CppPattern
// never panics, refuses only with ErrPattern, and its translation is printable ASCII that RE2,
// run over the text's bytes widened to Latin-1, accepts exactly when the pattern accepts the text.
func FuzzCppPattern(f *testing.F) {
	for i, p := range append(append([]string(nil), specPatterns...), robustPatterns...) {
		f.Add(p, fixedInputs[i%len(fixedInputs)])
	}
	f.Fuzz(func(t *testing.T, p, in string) {
		re, err := regexp.Compile(p)
		if err != nil || !utf8.ValidString(in) {
			return
		}
		tr, err := ir.CppPattern(re)
		if err != nil {
			if !errors.Is(err, ir.ErrPattern) {
				t.Fatalf("CppPattern(%q): %v", p, err)
			}
			return
		}
		if _, err := regexp.Compile(tr); tooLarge(err) {
			return
		}
		assertBytesAgree(t, re, tr, []string{in, in + in, strings.ToUpper(in)})
	})
}

// tooLarge is RE2 refusing a translation for its size alone: the byte classes of a counted
// repetition can pass RE2's limits where the pattern itself did not.
func tooLarge(err error) bool {
	var se *resyntax.Error
	return errors.As(err, &se) && (se.Code == resyntax.ErrLarge || se.Code == resyntax.ErrNestingDepth)
}
