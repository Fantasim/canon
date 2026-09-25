package ir_test

import (
	"context"
	"encoding/hex"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/ir"
	"github.com/fantasim/canonlang/internal/testkit/cxx"
)

// harnessLocales are the global locales the harness runs under: the translation reads none.
var harnessLocales = []string{"C", "C.utf8"}

// TestCppPatternStdRegex is log 2026-09-24 "Pattern semantics" against the C++ engine: for every
// pattern × input of the corpus, std::regex_search (ECMAScript, over the bytes) of the translation
// agrees with Go's regexp.MatchString of the pattern, per compiler found and harnessLocales entry.
func TestCppPatternStdRegex(t *testing.T) {
	compilers, _ := cxx.Toolchain(t)
	patterns, inputs := fullCorpus()
	stdin, want := harnessCase(t, patterns, inputs)
	dir := t.TempDir()
	for _, cc := range compilers {
		bin := filepath.Join(dir, filepath.Base(cc))
		args := append(append([]string(nil), cxx.Flags...), "-o", bin, filepath.Join("testdata", "pattern", "harness.cpp"))
		ctx, cancel := context.WithTimeout(context.Background(), cxx.Timeout)
		out, err := exec.CommandContext(ctx, cc, args...).CombinedOutput()
		cancel()
		if err != nil {
			t.Fatalf("%s: %v\n%s", filepath.Base(cc), err, out)
		}
		for _, loc := range harnessLocales {
			c := harnessRun{name: filepath.Base(cc) + " " + loc, patterns: patterns, inputs: inputs, want: want}
			c.compare(t, runHarness(t, bin, loc, stdin))
		}
	}
	t.Logf("%d patterns × %d inputs = %d pairs per compiler and locale", len(patterns), len(inputs), len(patterns)*len(inputs))
}

// harnessCase is the harness's stdin and, per pattern, Go's answers as the harness writes them.
func harnessCase(t *testing.T, patterns, inputs []string) (stdin string, want []string) {
	t.Helper()
	var b strings.Builder
	b.WriteString(strconv.Itoa(len(inputs)) + "\n")
	for _, in := range inputs {
		b.WriteString(hex.EncodeToString([]byte(in)) + "\n")
	}
	for _, p := range patterns {
		re := regexp.MustCompile(p)
		tr, err := ir.CppPattern(re)
		if err != nil {
			t.Fatalf("CppPattern(%q): %v", p, err)
		}
		b.WriteString(tr + "\n")
		row := make([]byte, len(inputs))
		for i, in := range inputs {
			row[i] = '0'
			if re.MatchString(in) {
				row[i] = '1'
			}
		}
		want = append(want, string(row))
	}
	return b.String(), want
}

// runHarness runs the compiled harness under locale loc and returns its rows.
func runHarness(t *testing.T, bin, loc, stdin string) []string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), cxx.Timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, loc)
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("harness %s: %v", loc, err)
	}
	return strings.Split(strings.TrimSuffix(string(out), "\n"), "\n")
}

// harnessRun is one run of the harness: its name, the corpus and Go's rows.
type harnessRun struct {
	name                   string
	patterns, inputs, want []string
}

// compare reports every pair where std::regex and Go disagree, and every refused pattern.
func (c harnessRun) compare(t *testing.T, got []string) {
	t.Helper()
	run, patterns, inputs, want := c.name, c.patterns, c.inputs, c.want
	if len(got) != len(want) {
		t.Fatalf("%s: %d rows, want %d", run, len(got), len(want))
	}
	for i, row := range got {
		if len(row) != len(inputs) {
			t.Errorf("%s: pattern %q: std::regex answered %q", run, patterns[i], row)
			continue
		}
		for j := range inputs {
			if row[j] != want[i][j] {
				t.Errorf("%s: pattern %q, input %q: std::regex %c, Go %c", run, patterns[i], inputs[j], row[j], want[i][j])
			}
		}
	}
}
