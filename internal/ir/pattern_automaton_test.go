package ir_test

import (
	"errors"
	"regexp"
	resyntax "regexp/syntax"
	"sort"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/fantasim/canonlang/internal/ir"
)

// simulate is gen/cpp's MatchPattern in Go (CODEGEN.md §7.7): one set of states per position, the start added at each (search semantics), a code point decoded as regexp decodes it.
func simulate(a ir.PatternAutomaton, s string) bool {
	cur, next := newStateSet(len(a.States)), newStateSet(len(a.States))
	for pos := 0; ; {
		if cur.add(a, 0, anchorsAt(pos, len(s))) {
			return true
		}
		if pos == len(s) {
			return false
		}
		r, w := utf8.DecodeRuneInString(s[pos:])
		next.reset()
		at := anchorsAt(pos+w, len(s))
		for _, i := range cur.list {
			st := a.States[i]
			if st.Op == ir.PatternStep && inRunes(st.Runes, r) && next.add(a, st.Out, at) {
				return true
			}
		}
		cur, next = next, cur
		pos += w
	}
}

// anchorsAt are the anchors that hold at pos of a text of n bytes.
func anchorsAt(pos, n int) ir.PatternAt {
	var at ir.PatternAt
	if pos == 0 {
		at |= ir.PatternAtBegin
	}
	if pos == n {
		at |= ir.PatternAtEnd
	}
	return at
}

// inRunes is a binary search of r in sorted lo, hi pairs.
func inRunes(pairs []rune, r rune) bool {
	n := len(pairs) / 2
	i := sort.Search(n, func(i int) bool { return pairs[2*i+1] >= r })
	return i < n && pairs[2*i] <= r
}

// stateSet is the states live at one position, in the order they were added.
type stateSet struct {
	list []int
	in   []bool
}

func newStateSet(n int) *stateSet { return &stateSet{in: make([]bool, n)} }

func (s *stateSet) reset() {
	for _, i := range s.list {
		s.in[i] = false
	}
	s.list = s.list[:0]
}

// add adds from and every state it reaches without consuming, with an explicit stack; true on accept.
func (s *stateSet) add(a ir.PatternAutomaton, from int, at ir.PatternAt) bool {
	var stack []int
	push := func(i int) {
		if !s.in[i] {
			s.in[i] = true
			s.list = append(s.list, i)
			stack = append(stack, i)
		}
	}
	push(from)
	for len(stack) > 0 {
		st := a.States[stack[len(stack)-1]]
		stack = stack[:len(stack)-1]
		switch st.Op {
		case ir.PatternAccept:
			return true
		case ir.PatternSplit:
			push(st.Out)
			push(st.Alt)
		case ir.PatternAnchor:
			if st.At&at == st.At {
				push(st.Out)
			}
		}
	}
	return false
}

// badUTF8 are texts regexp reads a byte at a time as U+FFFD: overlongs, surrogates, truncations, stray bytes.
var badUTF8 = []string{"\xC0\xAF", "\xE0\x80\xAF", "\xED\xA0\x80", "\xF4\x90\x80\x80", "\xFF", "\x80", "a\xC3", "\xE2\x82", "\xF0\x9F\x98", "\xC3\x28"}

// assertAutomatonAgrees fails when the automaton of p and regexp disagree on one input.
func assertAutomatonAgrees(t *testing.T, p string, inputs []string) {
	t.Helper()
	re := regexp.MustCompile(p)
	a, err := ir.CompilePattern(re)
	if err != nil {
		t.Fatalf("CompilePattern(%q): %v", p, err)
	}
	for _, in := range inputs {
		if want, got := re.MatchString(in), simulate(a, in); want != got {
			t.Errorf("pattern %q, input %q: regexp %v, automaton %v", p, in, want, got)
		}
	}
}

// TestPatternAutomatonConstructs is EVALUATION.md §11.3's portable subset, one construct per row, searched as Go's regexp.MatchString does (CODEGEN.md §5.12), valid and invalid UTF-8 alike.
func TestPatternAutomatonConstructs(t *testing.T) {
	cases := []struct{ name, pattern string }{
		{"literal", `abc_1`}, {"escaped metacharacters", `\.\*\+\?\(\)\[\]\{\}\|\^\$\\`}, {"multi-byte literal", `^é$`},
		{"dot, not a newline", `^.$`}, {"class with ranges", `^[a-c0-9_]+$`}, {"negated class", `^[^a]$`},
		{"non-ASCII class members", `[é-ü]`}, {"mixed class", `^[aé😀]$`}, {`\d`, `^\d+$`}, {`\D`, `^\D$`},
		{`\w`, `^\w+$`}, {`\W`, `^\W$`}, {`\s`, `^\s$`}, {`\S`, `^\S+$`}, {"anchors", `^a$`}, {"start anchor", `^a`},
		{"end anchor", `a$`}, {"unanchored search", `b`}, {"group", `(ab)c`}, {"non-capturing group", `^(?:ab)+$`},
		{"alternation", `^(?:ab|cd)$`}, {"alternation with anchors", `a$|^b`}, {"star", `^a*$`}, {"plus", `^a+$`},
		{"quest", `^ab?$`}, {"counted", `^a{2}$`}, {"counted, open", `^a{2,}$`}, {"counted, bounded", `^a{2,3}$`},
		{"lazy forms", `^a*?b+?c??d{1,2}?$`}, {"empty alternative", `^(a|)$`}, {"empty pattern", ``},
		{"nested stars", `^(a*)*$`}, {"anchor in a loop", `(^a)*b`}, {"any code point", `^[\s\S]$`},
		{"matches nothing", `[^\s\S]`}, {"surrogate literal (RE2 spelling)", `a\x{D800}`},
		{"U+FFFD", `^\x{FFFD}+$`}, {"not U+FFFD", `^[^\x{FFFD}]$`},
	}
	inputs := append(append([]string(nil), fixedInputs...), badUTF8...)
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) { assertAutomatonAgrees(t, c.pattern, inputs) })
	}
}

// TestPatternAutomatonCorpus is EVALUATION.md §11.3 over every corpus pair: the automaton accepts exactly what regexp accepts, and has no more states than Go's own program.
func TestPatternAutomatonCorpus(t *testing.T) {
	patterns, inputs := fullCorpus()
	inputs = append(inputs, badUTF8...)
	for _, p := range patterns {
		assertAutomatonAgrees(t, p, inputs)
		tree, err := resyntax.Parse(p, resyntax.Perl)
		if err != nil {
			t.Fatal(err)
		}
		prog, err := resyntax.Compile(tree.Simplify())
		if err != nil {
			t.Fatal(err)
		}
		if a, _ := ir.CompilePattern(regexp.MustCompile(p)); len(a.States) > len(prog.Inst) {
			t.Errorf("pattern %q: %d states, Go's program %d instructions", p, len(a.States), len(prog.Inst))
		}
	}
}

// automatonRefused are the patterns CompilePattern has no automaton for.
var automatonRefused = []string{`(?i)a`, `(?m)^a`, `(?m)a$`, `\ba`, `a\B`}

// TestPatternAutomatonRefuses is what has no state: case folding, multi-line anchors and word boundaries, outside EVALUATION.md §11.3's subset (E1904 refuses them first).
func TestPatternAutomatonRefuses(t *testing.T) {
	for _, p := range automatonRefused {
		if a, err := ir.CompilePattern(regexp.MustCompile(p)); !errors.Is(err, ir.ErrPattern) {
			t.Errorf("CompilePattern(%q) = %v, %v; want ErrPattern", p, a, err)
		}
	}
}

// envMax is the longest environment value EVALUATION.md §11.3 inputs meet: 128 KiB.
const envMax = 128 << 10

// TestPatternAutomatonLongInput is log 2026-09-25 "Pattern translator calls": a 128 KiB text, where libstdc++'s std::regex recursed per character, is searched like any other.
func TestPatternAutomatonLongInput(t *testing.T) {
	long := strings.Repeat("a", envMax)
	inputs := []string{long, long + "\n", strings.Repeat("é", envMax/2), strings.Repeat("x", envMax), long + "b"}
	for _, p := range []string{`^.*$`, `^(a|aa)*$`, `^(?:x+x+)+y$`, `^[^\n]+b$`, `(?:a|é)+$`} {
		assertAutomatonAgrees(t, p, inputs)
	}
}
