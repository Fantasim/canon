package cppgen_test

import (
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	cppgen "github.com/fantasim/canonlang/internal/gen/cpp"
	"github.com/fantasim/canonlang/internal/ir"
)

// cxxPatterns are EVALUATION.md §11.3's constructs alone and combined, and the shapes that made std::regex recurse or backtrack (log 2026-09-25 "Pattern translator calls").
var cxxPatterns = []string{
	`abc`, `^abc$`, `^.$`, `^.*$`, `^.+$`, `^[a-z]+$`, `^[^a-z]+$`, `[^a]`, `^\d+$`, `^\D$`, `^\w+$`,
	`^\W$`, `^\s$`, `^\S+$`, `^(?:ab|cd){2,3}$`, `a{2,}`, `^a{2,3}$`, `a*?b`, `^(a|)$`, `^$`, `\.`,
	`\*\+\?\(\)\[\]\{\}\|\^\$\\`, `^é$`, `^[à-ÿ]+$`, `^[^é]$`, `^[😀-🙏]$`, `日本`, `^[a-zé€😀]{2}$`,
	`a|^b`, `a$|b`, `(^a)*b`, `(a|b*)*c`, `^\d{3}-\d{4}$`, `^[A-Za-z_][A-Za-z0-9_]*$`, `^\S+@\S+\.\S+$`,
	`[^\s\S]`, `^[\s\S]$`, `^(a|aa)*$`, `^(?:x+x+)+y$`, `^[^\n]+b$`, `^\x{FFFD}$`,
}

// cxxTexts are texts the constructs tell apart: ASCII, every UTF-8 length, spaces RE2's `\s` has
// and has not, invalid UTF-8, and 128 KiB values, the environment's bound.
var cxxTexts = []string{
	"", "a", "b", "abc", "xabcx", "abcd", "abab", "cdcdcd", "aa", "aaa", "aaaa", "aab", "é", "àÿ", "日本",
	"日本語", "😀", "🙏", "€", "é€", "a😀", "\n", "\r", "\t", "\v", "\f", " ", " ", "a\nb", "\x7f",
	"\u0080", "\U0010ffff", "�", "_", "-", "123", "555-1234", "1a", "a@b.c", "x.y", "*+?()[]{}|^$\\",
	"ac", "bbbc", "\xFF", "\xC3\x28", "a\xE2\x82", strings.Repeat("a", envMax), strings.Repeat("a", envMax-1) + "\n",
	strings.Repeat("a", envMax-1) + "b", strings.Repeat("x", envMax), strings.Repeat("é", envMax/2),
}

// envMax is the longest environment value inputs meet: 128 KiB.
const envMax = 128 << 10

// loaderPatterns are cxxPatterns and a few seeded patterns of the subset, one input each.
var loaderPatterns = append(slices.Clone(cxxPatterns), seededPatterns(8)...)

// patternsPackage is a record Config with one optional String input per loaderPatterns entry.
func patternsPackage() (*ir.Package, []string) {
	rec := &ir.Record{Pkg: "pat", Name: "Config"}
	var vars []string
	for i, p := range loaderPatterns {
		v := fmt.Sprintf("CANON_PAT_%d", i)
		f := input(fmt.Sprintf("p%d", i), v, tString, true, nil)
		f.Patterns = []*regexp.Regexp{regexp.MustCompile(p)}
		rec.Fields = append(rec.Fields, f)
		vars = append(vars, v)
	}
	emit := &ir.Emit{Target: ir.TargetCpp, Dir: "pat/out", Mode: ir.ModeData, Namespace: "pat"}
	return &ir.Package{Name: "pat", Dir: "pat", Types: []ir.Type{rec}, Emits: []*ir.Emit{emit}}, vars
}

// goMarks is the Go loader's answer per pattern for text s (CODEGEN.md §5.12): unset when empty, not a String when invalid UTF-8, else regexp.MatchString's search.
func goMarks(s string) string {
	row := make([]byte, len(loaderPatterns))
	for i, p := range loaderPatterns {
		switch {
		case s == "":
			row[i] = 'o'
		case !utf8.ValidString(s):
			row[i] = 'x'
		case regexp.MustCompile(p).MatchString(s):
			row[i] = 'o'
		default:
			row[i] = 'p'
		}
	}
	return string(row)
}

// TestPatternInputsCompileAndRun is EVALUATION.md §11.3 and CODEGEN.md §5.12, §7.7: with every toolchain, MatchPattern over ir's automaton accepts exactly the texts the Go loader accepts, 128 KiB ones included; a patterned input adds MatchPattern to the helpers.
func TestPatternInputsCompileAndRun(t *testing.T) {
	t.Parallel()
	p, vars := patternsPackage()
	files := generate(t, p)
	want := []string{"EnvText", "ParseStringLiteral", ir.CppMatchPattern, "LoadInputs"}
	if got := helpersIn(files, "pat.gen.cpp"); !slices.Equal(got, want) {
		t.Errorf("helpers %v, want %v", got, want)
	}
	dir := t.TempDir()
	writeTree(t, dir, files, "patterns_main.cpp")
	var list, rows strings.Builder
	list.WriteString(strings.Join(vars, " ") + "\n")
	for _, s := range cxxTexts {
		list.WriteString(hex.EncodeToString([]byte(s)) + "\n")
		rows.WriteString(goMarks(s) + "\n")
	}
	file := filepath.Join(dir, "texts.txt")
	if err := os.WriteFile(file, []byte(list.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, out := range buildAndRun(t, dir, []string{"main.cpp", "pat.gen.cpp"}, file) {
		compareMarks(t, strings.Split(out, "\n"), strings.Split(rows.String(), "\n"))
	}
}

// compareMarks reports every text and pattern where the C++ rows and Go's disagree.
func compareMarks(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%d rows, want %d", len(got), len(want))
	}
	for i := range cxxTexts {
		if len(got[i]) != len(loaderPatterns) {
			t.Fatalf("text %.40q: row %q", cxxTexts[i], got[i])
		}
		for j := range loaderPatterns {
			if got[i][j] != want[i][j] {
				t.Errorf("pattern %q, text %.40q: C++ %c, Go %c", loaderPatterns[j], cxxTexts[i], got[i][j], want[i][j])
			}
		}
	}
}

// TestPatternOutsideSubsetIsMalformed is CODEGEN.md §5.12: a pattern ir has no automaton for (EVALUATION.md §11.3; E1904 refuses it first) is malformed IR, never a guessed matcher.
func TestPatternOutsideSubsetIsMalformed(t *testing.T) {
	for _, p := range []string{`(?i)a`, `\ba`, `(?m)^a`} {
		pkg := oneInput("outside", tString)
		pkg.Types[0].(*ir.Record).Fields[0].Patterns = []*regexp.Regexp{regexp.MustCompile(p)}
		_, err := cppgen.Generate(pkg, pkg.Emits[0])
		if !errors.Is(err, cppgen.ErrMalformed) || !strings.Contains(err.Error(), "an input pattern outside") {
			t.Errorf("pattern %q: %v, want ErrMalformed naming the pattern", p, err)
		}
	}
}
