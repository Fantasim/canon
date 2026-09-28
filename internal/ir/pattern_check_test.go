package ir_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/ir"
)

// TestCompilePatternAcceptsWhatCheckAccepts is EVALUATION.md §11.3 end to end: every corpus pattern passes check (no E1904) and compiles to an automaton, and every pattern CompilePattern refuses is E1904 first, so generation never meets one it cannot write.
func TestCompilePatternAcceptsWhatCheckAccepts(t *testing.T) {
	genPats, _ := generatedCorpus()
	for _, p := range append(append([]string(nil), specPatterns...), genPats...) {
		refused, pat := checkPattern(t, p)
		switch {
		case refused:
			t.Errorf("pattern %q: refused by check, but it is in the portable subset", p)
		case pat == nil:
			t.Errorf("pattern %q: no input pattern in the IR", p)
		default:
			if _, err := ir.CompilePattern(pat); err != nil {
				t.Errorf("pattern %q: check accepts it, CompilePattern: %v", p, err)
			}
		}
	}
	for _, p := range automatonRefused {
		if refused, _ := checkPattern(t, p); !refused {
			t.Errorf("pattern %q: check lets it through, but CompilePattern refuses it", p)
		}
	}
}

// checkPattern builds one input field holding pattern p: whether check reported E1904, and
// the field's IR pattern.
func checkPattern(t *testing.T, p string) (refused bool, pat *regexp.Regexp) {
	t.Helper()
	w := newWorld(t)
	w.add(t, "a/a.canon", []byte("package a\n\n/// S.\nrecord S {\n  /// P.\n  p: input String(/"+
		strings.ReplaceAll(p, "/", `\/`)+"/)? from env \"P\"\n}\n\n/// V.\nlet v: S = {}\n\nemit go { out: \"@features/a\", package: \"a\" }\n"))
	w.add(t, "a.v.json", []byte("{}"))
	for _, pkg := range w.build(t) {
		for _, typ := range pkg.Types {
			if r, ok := typ.(*ir.Record); ok && len(r.Fields) > 0 && len(r.Fields[0].Patterns) == 1 {
				pat = r.Fields[0].Patterns[0]
			}
		}
	}
	return strings.Contains(w.findings(t), string(diag.E1904.Def().Code)), pat
}
