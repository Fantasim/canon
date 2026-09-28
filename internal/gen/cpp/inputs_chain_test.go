package cppgen_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
)

// chainPatterns are an alias chain's patterns, innermost first: `type Code = String(/^[a-z]+$/)`,
// `type Short = Code(/^.{1,4}$/)`, and a third written on the field (`Short(/^a/)`).
var chainPatterns = []string{`^[a-z]+$`, `^.{1,4}$`, `^a`}

// chainPackage is demo.chain: Config.code is an input of type Short (two patterns), Config.tag
// one of Short(/^a/) (three), with a C++ and a Go data emit.
func chainPackage() (*ir.Package, *ir.Emit) {
	patterns := func(n int) []*regexp.Regexp {
		var out []*regexp.Regexp
		for _, p := range chainPatterns[:n] {
			out = append(out, regexp.MustCompile(p))
		}
		return out
	}
	code := input("code", "CANON_CHAIN_CODE", tString, true, nil)
	code.Patterns = patterns(len(chainPatterns) - 1)
	tag := input("tag", "CANON_CHAIN_TAG", tString, true, nil)
	tag.Patterns = patterns(len(chainPatterns))
	rec := &ir.Record{Pkg: "demo.chain", Name: "Config", Fields: []*ir.Field{code, tag}}
	goEmit := &ir.Emit{Target: ir.TargetGo, Out: "out/go/", Dir: "demo/chain/out/go", GoImport: parityModule + "/chain", Mode: ir.ModeData, GoPackage: "chain"}
	cppEmit := &ir.Emit{Target: ir.TargetCpp, Out: "out/", Dir: "demo/chain/out", Mode: ir.ModeData, Namespace: "demo::chain"}
	return &ir.Package{Name: "demo.chain", Dir: "demo/chain", Types: []ir.Type{rec}, Emits: []*ir.Emit{cppEmit, goEmit}}, goEmit
}

// chainRows are texts set on both variables, and the line both drivers print for them.
var chainRows = []struct{ text, want string }{
	{"abc", "code=abc tag=abc err="},
	{"abcd", "code=abcd tag=abcd err="},
	// Passes the outer pattern, fails the inner one.
	{"ABC", "code=none tag=none err=CANON_CHAIN_CODE: does not match its pattern | CANON_CHAIN_TAG: does not match its pattern"},
	// Passes the inner pattern, fails the outer one.
	{"abcdef", "code=none tag=none err=CANON_CHAIN_CODE: does not match its pattern | CANON_CHAIN_TAG: does not match its pattern"},
	// Passes both of code's, fails only tag's own.
	{"bcd", "code=bcd tag=none err=CANON_CHAIN_TAG: does not match its pattern"},
	// Fails every pattern: still one line per variable.
	{"ABCDEF", "code=none tag=none err=CANON_CHAIN_CODE: does not match its pattern | CANON_CHAIN_TAG: does not match its pattern"},
}

// TestInputPatternChain is TYPES.md §7.4 ("both are checked"), EVALUATION.md §11.3 step 3 and CODEGEN.md §7.7: every pattern of the chain gets its own table, kPattern then kPattern2, kPattern3, checked innermost first.
func TestInputPatternChain(t *testing.T) {
	p, _ := chainPackage()
	var src string
	for _, f := range generate(t, p) {
		if f.Path == "chain.gen.cpp" {
			src = string(f.Content)
		}
	}
	tables := regexp.MustCompile(`static constexpr uint32_t (kPattern\d*)\[\]`).FindAllStringSubmatch(src, -1)
	var names []string
	for _, m := range tables {
		names = append(names, m[1])
	}
	if want := "kPattern kPattern2 kPattern kPattern2 kPattern3"; strings.Join(names, " ") != want {
		t.Errorf("tables %v, want %s", names, want)
	}
	first, second := strings.Index(src, "!MatchPattern(kPattern, val)"), strings.Index(src, "!MatchPattern(kPattern2, val)")
	if first < 0 || second < first {
		t.Errorf("want kPattern checked before kPattern2:\n%s", src)
	}
}

// TestInputPatternChainParity is EVALUATION.md §11.3 step 3 and CODEGEN.md §5.12, §7.7: the Go and C++ loaders refuse a text that fails any pattern of the chain, the outer one or an inner one, with the same line, and accept the same texts.
func TestInputPatternChainParity(t *testing.T) {
	t.Parallel()
	if testing.Short() {
		t.Skip("compiles generated Go and C++")
	}
	p, goEmit := chainPackage()
	args := make([]string, len(chainRows))
	for i, r := range chainRows {
		args[i] = r.text
	}
	check := func(target, out string) {
		got := strings.Split(strings.TrimSuffix(out, "\n"), "\n")
		if len(got) != len(chainRows) {
			t.Fatalf("%s: %d lines for %d texts:\n%s", target, len(got), len(chainRows), out)
		}
		for i, r := range chainRows {
			if got[i] != r.want {
				t.Errorf("%s: %q:\n got %s\nwant %s", target, r.text, got[i], r.want)
			}
		}
	}
	check("go", runGo(t, []parityUnit{{pkg: p, goEmit: goEmit}}, "chain_main.go", args))
	dir := t.TempDir()
	writeTree(t, dir, generate(t, p), "chain_main.cpp")
	for _, out := range buildAndRun(t, dir, []string{"main.cpp", "chain.gen.cpp"}, args...) {
		check("c++", out)
	}
}
