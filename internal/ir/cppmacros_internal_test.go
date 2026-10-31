package ir

import (
	"os"
	"slices"
	"strings"
	"testing"
)

const (
	macroSpec       = "../../spec/CODEGEN.md"
	macroSpecAnchor = "in byte order:"
	macroSpecEnd    = "\n\n- **TS**"
)

// TestCppMacroFile is CODEGEN.md §3.4: both lists sorted, duplicate-free, holding the decision's examples and disjoint.
func TestCppMacroFile(t *testing.T) {
	std, warn := parseMacroLists(cppMacroText)
	for name, list := range map[string][]string{"[std]": std, "[warn]": warn} { //canon:unordered each list alone
		if !slices.IsSorted(list) {
			t.Errorf("%s is not sorted", name)
		}
		if got := slices.Compact(slices.Clone(list)); len(got) != len(list) {
			t.Errorf("%s holds %d duplicate(s)", name, len(list)-len(got))
		}
	}
	for _, want := range strings.Fields(`INT32_MAX UINT32_MAX SIZE_MAX INT64_C EOF NULL BUFSIZ SEEK_SET stdin stdout stderr
EXIT_SUCCESS RAND_MAX HUGE_VAL INFINITY NAN FP_NAN math_errhandling errno EDOM ERANGE EWOULDBLOCK DBL_MAX FLT_EPSILON
LC_ALL offsetof ATOMIC_FLAG_INIT`) {
		if !cppStdMacros[want] {
			t.Errorf("[std] lacks %s", want)
		}
	}
	for _, n := range warn {
		if cppStdMacros[n] {
			t.Errorf("%s is in both macro lists", n)
		}
	}
}

// TestCppStdMacrosAreTheSpecs is CODEGEN.md §3.4: the names after "in byte order:" are the [std] section.
func TestCppStdMacrosAreTheSpecs(t *testing.T) {
	data, err := os.ReadFile(macroSpec)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	i := strings.Index(text, macroSpecAnchor)
	if i < 0 {
		t.Fatalf("%s has no %q", macroSpec, macroSpecAnchor)
	}
	rest := text[i+len(macroSpecAnchor):]
	end := strings.Index(rest, macroSpecEnd)
	if end < 0 {
		t.Fatalf("%s: the list does not end at %q", macroSpec, macroSpecEnd)
	}
	want, _ := parseMacroLists(cppMacroText)
	if got := strings.Fields(rest[:end]); !slices.Equal(got, want) {
		t.Errorf("CODEGEN.md §3.4 lists %d names, [std] holds %d; only in the spec %v, only in [std] %v", len(got), len(want), onlyIn(got, want), onlyIn(want, got))
	}
}

// TestCppMacroWarnList is CODEGEN.md §3.5: W8006 holds <climits>'s names, NAME_MAX, PATH_MAX and the M_* constants.
func TestCppMacroWarnList(t *testing.T) {
	for _, want := range strings.Fields(`CHAR_BIT SCHAR_MIN SCHAR_MAX UCHAR_MAX CHAR_MIN CHAR_MAX MB_LEN_MAX SHRT_MIN SHRT_MAX
USHRT_MAX INT_MIN INT_MAX UINT_MAX LONG_MIN LONG_MAX ULONG_MAX LLONG_MIN LLONG_MAX ULLONG_MAX NAME_MAX PATH_MAX M_E M_LOG2E
M_LOG10E M_LN2 M_LN10 M_PI M_PI_2 M_PI_4 M_1_PI M_2_PI M_2_SQRTPI M_SQRT2 M_SQRT1_2 min max GetObject`) {
		if !cppMacros[want] {
			t.Errorf("[warn] lacks %s", want)
		}
	}
}

// TestCppValidIdentMacro is CODEGEN.md §3.4: a standard macro is refused as reserved, case-sensitively.
func TestCppValidIdentMacro(t *testing.T) {
	cases := []struct {
		name         string
		ok, reserved bool
	}{
		{"UINT32_MAX", false, true},
		{"errno", false, true},
		{"Errno", true, false},
		{"uint32_max", true, false},
		{"kUint32Max", true, false},
	}
	for _, c := range cases {
		if ok, reserved := cppValidIdent(c.name); ok != c.ok || reserved != c.reserved {
			t.Errorf("cppValidIdent(%q) = %v, %v, want %v, %v", c.name, ok, reserved, c.ok, c.reserved)
		}
	}
}

// onlyIn are the names of a that b lacks.
func onlyIn(a, b []string) []string {
	return slices.DeleteFunc(slices.Clone(a), func(n string) bool { return slices.Contains(b, n) })
}
