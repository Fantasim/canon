package cppgen_test

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/fantasim/canonlang/internal/diag"
)

// inputCase is one variable's text and the failure line LoadInputs writes for it ("" when read).
type inputCase struct{ env, text, reason string }

// inputCases are EVALUATION.md §11.3's texts, each variable read alone, with ir's reasons (a Float32 reads Float; an enum its Canon name).
var inputCases = []inputCase{
	{"CANON_TEST_PORT", "5000000000", "outside its refinement range"}, {"CANON_TEST_PORT", "70000", "outside its refinement range"},
	{"CANON_TEST_PORT", "0x10", "not a valid Int"}, {"CANON_TEST_PORT", "+5", "not a valid Int"}, {"CANON_TEST_PORT", "065535", ""},
	{"CANON_TEST_COUNT", "9223372036854775807", ""}, {"CANON_TEST_COUNT", "-9223372036854775808", ""},
	{"CANON_TEST_COUNT", "9223372036854775808", "not a valid Int"}, {"CANON_TEST_COUNT", "1_000", "not a valid Int"},
	{"CANON_TEST_COUNT", "-", "not a valid Int"}, {"CANON_TEST_COUNT", " 1", "not a valid Int"},
	{"CANON_TEST_DEBUG", "maybe", "not a valid Bool"}, {"CANON_TEST_DEBUG", "True", "not a valid Bool"}, {"CANON_TEST_DEBUG", "false", ""},
	{"CANON_TEST_NAME", "abcdefghijk", "outside its refinement range"}, {"CANON_TEST_NAME", "é", ""},
	{"CANON_TEST_TAG", "ab", "does not match its pattern"}, {"CANON_TEST_TAG", "XYZ", ""},
	{"CANON_TEST_TIMEOUT", "not-a-duration", "not a valid Duration"}, {"CANON_TEST_TIMEOUT", "2m", "outside its refinement range"},
	{"CANON_TEST_TIMEOUT", "1s500ms", ""}, {"CANON_TEST_WAIT", "99999999999999d", "not a valid Duration"},
	{"CANON_TEST_WAIT", "9223372036854ms", ""}, {"CANON_TEST_WAIT", "9223372036855ms", "not a valid Duration"},
	{"CANON_TEST_WAIT", "-1d2h3m4s5ms", ""}, {"CANON_TEST_WAIT", "1_000ms", ""}, {"CANON_TEST_WAIT", "1__0s", "not a valid Duration"},
	{"CANON_TEST_WAIT", "_1s", "not a valid Duration"}, {"CANON_TEST_WAIT", "1m1h", "not a valid Duration"},
	{"CANON_TEST_WAIT", "1s1s", "not a valid Duration"}, {"CANON_TEST_WAIT", "5", "not a valid Duration"},
	{"CANON_TEST_WAIT", "1.5s", "not a valid Duration"}, {"CANON_TEST_WAIT", "-", "not a valid Duration"},
	{"CANON_TEST_RATE", "not-a-number", "not a valid Float"}, {"CANON_TEST_RATE", "1e40", "outside its refinement range"},
	{"CANON_TEST_RATE", "2.0", "outside its refinement range"}, {"CANON_TEST_RATE", "1.00000001", ""},
	{"CANON_TEST_RATIO", "1e-400", ""}, {"CANON_TEST_RATIO", "4e-320", ""}, {"CANON_TEST_RATIO", "-0", ""},
	{"CANON_TEST_RATIO", "00012", ""}, {"CANON_TEST_RATIO", "1E+5", ""}, {"CANON_TEST_RATIO", "1e400", "not a valid Float"},
	{"CANON_TEST_RATIO", "1.", "not a valid Float"}, {"CANON_TEST_RATIO", ".5", "not a valid Float"},
	{"CANON_TEST_RATIO", "+1", "not a valid Float"}, {"CANON_TEST_RATIO", "inf", "not a valid Float"},
	{"CANON_TEST_RATIO", "1e", "not a valid Float"}, {"CANON_TEST_RATIO", "1,5", "not a valid Float"},
	{"CANON_TEST_COLOR", "blue", "not a member of Color"}, {"CANON_TEST_COLOR", "Red", "not a member of Color"},
	{"CANON_TEST_LEVEL", "high", ""}, {"CANON_TEST_LEVEL", "2", "not a member of Level"},
}

// utf8Texts are utf8.ValidString's edge cases (EVALUATION.md §11.3): overlongs, surrogates, past U+10FFFF, truncations, stray bytes.
var utf8Texts = []string{
	"plain", "é", "€", "\U0001F600", "\U0010FFFF", "�", "\xC0\xAF", "\xC1\xBF", "\xE0\x80\xAF", "\xE0\x9F\xBF",
	"\xF0\x80\x80\xAF", "\xF0\x8F\xBF\xBF", "\xED\xA0\x80", "\xED\xBF\xBF", "\xED\x9F\xBF", "\xF4\x90\x80\x80", "\xF5\x80\x80\x80",
	"\xFF", "\x80", "a\xC3", "\xE2\x82", "\xF0\x9F\x98", "\xC3\x28", "\xE2\x28\xA1",
}

// wantInputs is inputs_main.cpp's output: the scenarios, then one line per case.
func wantInputs(cases []inputCase) string {
	v := diag.E8302.Def().Variants[0]
	message := strings.ReplaceAll(v.Template, "{"+v.Args[0].Name+"}", "demo.Config.port")
	var b strings.Builder
	fmt.Fprintf(&b, "before: code=%s message=%s zero=1\n", diag.E8302.Def().Code, message)
	b.WriteString("missing: ok=0 err=CANON_TEST_PORT: not set\nCANON_TEST_DEBUG: not set\n")
	b.WriteString("happy: ok=1 port=8080 debug=1 name=hi tag=ABC timeoutMs=5000 rate=0.50 color=1 level=1\n")
	b.WriteString("reread: ok=1 port=9090 err=\nfailedreread: ok=0 port=0 name=none\n")
	for _, c := range cases {
		line := "ok"
		if c.reason != "" {
			line = c.env + ": " + c.reason
		}
		fmt.Fprintf(&b, "%s %s %s\n", c.env, hex.EncodeToString([]byte(c.text)), line)
	}
	return b.String()
}

// CODEGEN.md §5.12, §7.7; EVALUATION.md §11.3: inputs compiled and run with every toolchain; strings are UTF-8 as utf8.ValidString reads them.
func TestRuntimeInputsCompilesAndRuns(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, generate(t, inputsPackage()), "inputs_main.cpp")
	cases := append([]inputCase(nil), inputCases...)
	for _, s := range utf8Texts {
		reason := ""
		if !utf8.ValidString(s) {
			reason = "not a valid String"
		}
		cases = append(cases, inputCase{"CANON_TEST_NOTE", s, reason})
	}
	var list strings.Builder
	for _, c := range cases {
		fmt.Fprintf(&list, "%s %s\n", c.env, hex.EncodeToString([]byte(c.text)))
	}
	file := filepath.Join(dir, "cases.txt")
	if err := os.WriteFile(file, []byte(list.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	want := wantInputs(cases)
	for _, out := range buildAndRun(t, dir, []string{"main.cpp", "demo.gen.cpp"}, file) {
		if out != want {
			t.Errorf("got:\n%s\nwant:\n%s", out, want)
		}
	}
}
