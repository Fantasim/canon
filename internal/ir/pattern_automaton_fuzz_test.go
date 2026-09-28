package ir_test

import (
	"errors"
	"math/rand/v2"
	"regexp"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
)

// FuzzPatternAutomaton is EVALUATION.md §11.3 on any pattern Go compiles and any text, valid UTF-8 or not: CompilePattern never panics, refuses only with ErrPattern, and its automaton accepts exactly what regexp.MatchString accepts.
func FuzzPatternAutomaton(f *testing.F) {
	for i, p := range append(append([]string(nil), specPatterns...), robustPatterns...) {
		f.Add(p, fixedInputs[i%len(fixedInputs)])
	}
	f.Fuzz(func(t *testing.T, p, in string) {
		re, err := regexp.Compile(p)
		if err != nil {
			return
		}
		a, err := ir.CompilePattern(re)
		if err != nil {
			if !errors.Is(err, ir.ErrPattern) {
				t.Fatalf("CompilePattern(%q): %v", p, err)
			}
			return
		}
		for _, s := range []string{in, in + in, strings.ToUpper(in)} {
			if want, got := re.MatchString(s), simulate(a, s); want != got {
				t.Fatalf("pattern %q, input %q: regexp %v, automaton %v", p, s, want, got)
			}
		}
	})
}

// FuzzPatternAutomatonSubset is EVALUATION.md §11.3 on random patterns of the subset alone, drawn from seed: CompilePattern accepts every one, and its automaton agrees with regexp.
func FuzzPatternAutomatonSubset(f *testing.F) {
	for i, in := range fixedInputs {
		f.Add(uint64(i), in)
	}
	f.Fuzz(func(t *testing.T, seed uint64, in string) {
		p := genPattern(rand.New(rand.NewPCG(seed, genSeed2)), genMaxDepth)
		re := regexp.MustCompile(p)
		a, err := ir.CompilePattern(re)
		if err != nil {
			t.Fatalf("CompilePattern(%q): %v", p, err)
		}
		if want, got := re.MatchString(in), simulate(a, in); want != got {
			t.Fatalf("pattern %q, input %q: regexp %v, automaton %v", p, in, want, got)
		}
	})
}
