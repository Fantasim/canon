package cppgen_test

import (
	"cmp"
	"context"
	"encoding/hex"
	"fmt"
	"math/rand/v2"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	cppgen "github.com/fantasim/canonlang/internal/gen/cpp"
	"github.com/fantasim/canonlang/internal/testkit/cxx"
)

// seedAtoms, seedShapes and seedQuants build seededPatterns: §11.3's atoms, then two parts joined by a shape.
var (
	seedAtoms  = []string{`a`, `b`, `é`, `😀`, `.`, `\.`, `[a-c]`, `[^a]`, `[^é]`, `\d`, `\D`, `\w`, `\W`, `\s`, `\S`, `[é-ü]`, ` `, `^`, `$`}
	seedShapes = []string{`%s%s`, `(?:%s|%s)`, `(%s)*%s`, `^%s%s`, `%s%s$`, `(?:%s)+?%s`, `(?:%s){0,2}%s`, `%s?%s`, `(?:%s|)%s`}
)

// seededPatterns are n seeded RE2 patterns over §11.3's atoms, bare quantified anchors included.
func seededPatterns(n int) []string {
	r := rand.New(rand.NewPCG(20260928, 7))
	out := make([]string, n)
	for i := range out {
		out[i] = seeded(r, 3)
	}
	return out
}

func seeded(r *rand.Rand, depth int) string {
	if depth == 0 {
		return seedAtoms[r.IntN(len(seedAtoms))]
	}
	return fmt.Sprintf(seedShapes[r.IntN(len(seedShapes))], seeded(r, depth-1), seeded(r, depth-1))
}

// seedBytes are the letters of the random texts: ASCII, pieces of é, €, 😀 and U+FFFD, a surrogate lead, stray bytes.
var seedBytes = []byte{'a', 'b', '\n', ' ', '1', 0x00, 0x80, 0xBF, 0xC2, 0xC3, 0xA9, 0xE2, 0x82, 0xAC, 0xF0, 0x9F, 0x98, 0xED, 0xA0, 0xEF, 0xBD, 0xFF, 0xF4, 0x90}

// matcherTexts are raw bytes the decoder must read as Go's regexp does: every byte, every two bytes
// led by 0xC0–0xFF, three and four bytes around the E0, ED, F0 and F4 bounds, seeded mixes, 128 KiB values.
func matcherTexts() []string {
	var out []string
	for b := range 256 {
		out = append(out, string([]byte{byte(b)}))
		for c := range 256 {
			if b >= 0xC0 {
				out = append(out, string([]byte{byte(b), byte(c)}))
			}
		}
	}
	conts := []byte{0x7F, 0x80, 0x8F, 0x90, 0x9F, 0xA0, 0xBF, 0xC0}
	for _, lead := range []byte{0xE0, 0xED, 0xEF, 0xF0, 0xF4, 0xF5} {
		for _, c1 := range conts {
			for _, c2 := range conts {
				out = append(out, string([]byte{lead, c1, c2}), string([]byte{lead, c1, c2, 0x80}), string([]byte{lead, c1, c2, 0xC0}))
			}
		}
	}
	r := rand.New(rand.NewPCG(20260928, 11))
	for range 300 {
		b := make([]byte, 1+r.IntN(8))
		for i := range b {
			b[i] = seedBytes[r.IntN(len(seedBytes))]
		}
		out = append(out, string(b))
	}
	return append(out, "", strings.Repeat("\xFF", envMax), strings.Repeat("�", envMax/3))
}

// TestMatchPatternRawBytes is CODEGEN.md §7.7 and EVALUATION.md §11.3 on the matcher alone: over raw bytes, valid UTF-8 or not, MatchPattern on gen/cpp's table agrees with regexp.MatchString.
func TestMatchPatternRawBytes(t *testing.T) {
	t.Parallel()
	compilers := cxx.Compilers(t)
	patterns := append(append([]string(nil), cxxPatterns...), seededPatterns(200)...)
	texts := matcherTexts()
	var in, want strings.Builder
	fmt.Fprintf(&in, "%d %d\n", len(patterns), len(texts))
	for _, p := range patterns {
		data, err := cppgen.PatternData(regexp.MustCompile(p))
		if err != nil {
			t.Fatalf("pattern %q: %v", p, err)
		}
		fmt.Fprint(&in, len(data))
		for _, n := range data {
			fmt.Fprintf(&in, " %d", n)
		}
		in.WriteString("\n")
	}
	for _, s := range texts {
		in.WriteString(cmp.Or(hex.EncodeToString([]byte(s)), "-") + "\n")
	}
	for _, p := range patterns {
		want.WriteString(goRow(regexp.MustCompile(p), texts) + "\n")
	}
	bin := compileMatcher(t, compilers)
	for _, b := range bin {
		compareRows(t, runMatcher(t, b, in.String()), want.String(), patterns, texts)
	}
}

// goRow is regexp's answer per text, '1' or '0'.
func goRow(re *regexp.Regexp, texts []string) string {
	row := make([]byte, len(texts))
	for i, s := range texts {
		row[i] = '0'
		if re.MatchString(s) {
			row[i] = '1'
		}
	}
	return string(row)
}

// compileMatcher builds testdata/main/matcher_main.cpp around text/input/MatchPattern.txt once per compiler.
func compileMatcher(t *testing.T, compilers []string) []string {
	t.Helper()
	dir := t.TempDir()
	var bins []string
	for _, cc := range compilers {
		bin := filepath.Join(dir, filepath.Base(cc))
		args := append(append([]string(nil), cxx.Flags...), "-I", filepath.Join("text", "input"), "-o", bin, filepath.Join("testdata", "main", "matcher_main.cpp"))
		if out, err := compile(cc, args, ""); err != nil {
			t.Fatalf("%s: %v\n%s", filepath.Base(cc), err, out)
		}
		bins = append(bins, bin)
	}
	return bins
}

// runMatcher runs one build of the matcher on stdin.
func runMatcher(t *testing.T, bin, stdin string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), cxx.Timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin)
	cmd.Stdin = strings.NewReader(stdin)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("%s: %v", bin, err)
	}
	return string(out)
}

// compareRows reports every pattern and text where the matcher and regexp disagree.
func compareRows(t *testing.T, got, want string, patterns, texts []string) {
	t.Helper()
	g, w := strings.Split(got, "\n"), strings.Split(want, "\n")
	if len(g) != len(w) {
		t.Fatalf("%d rows, want %d", len(g), len(w))
	}
	for i, p := range patterns {
		if len(g[i]) != len(texts) {
			t.Fatalf("pattern %q: row of %d", p, len(g[i]))
		}
		for j := range texts {
			if g[i][j] != w[i][j] {
				t.Errorf("pattern %q, text %.40q: MatchPattern %c, regexp %c", p, texts[j], g[i][j], w[i][j])
			}
		}
	}
}
