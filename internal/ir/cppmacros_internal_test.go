package ir

import (
	"slices"
	"strings"
	"testing"
)

// TestCppStdMacroTable is CODEGEN.md §3.4: sorted, no duplicate, the decision's examples, disjoint from W8006's list.
func TestCppStdMacroTable(t *testing.T) {
	if !slices.IsSorted(cppStdMacroNames) {
		t.Errorf("cppStdMacroNames is not sorted")
	}
	if got := slices.Compact(slices.Clone(cppStdMacroNames)); len(got) != len(cppStdMacroNames) {
		t.Errorf("cppStdMacroNames holds %d duplicate(s)", len(cppStdMacroNames)-len(got))
	}
	for _, want := range strings.Fields(`INT32_MAX UINT32_MAX SIZE_MAX INT64_C EOF NULL BUFSIZ SEEK_SET stdin stdout stderr
EXIT_SUCCESS RAND_MAX HUGE_VAL INFINITY NAN FP_NAN math_errhandling errno EDOM ERANGE EWOULDBLOCK DBL_MAX FLT_EPSILON
LC_ALL offsetof ATOMIC_FLAG_INIT`) {
		if !cppStdMacros[want] {
			t.Errorf("cppStdMacroNames lacks %s", want)
		}
	}
	for _, n := range cppMacroNames {
		if cppStdMacros[n] {
			t.Errorf("%s is in both macro lists", n)
		}
	}
}

// TestCppMacroWarnList is CODEGEN.md §3.5: W8006 holds <climits>'s names, NAME_MAX, PATH_MAX and the M_* constants.
func TestCppMacroWarnList(t *testing.T) {
	for _, want := range strings.Fields(`CHAR_BIT SCHAR_MIN SCHAR_MAX UCHAR_MAX CHAR_MIN CHAR_MAX MB_LEN_MAX SHRT_MIN SHRT_MAX
USHRT_MAX INT_MIN INT_MAX UINT_MAX LONG_MIN LONG_MAX ULONG_MAX LLONG_MIN LLONG_MAX ULLONG_MAX NAME_MAX PATH_MAX M_E M_LOG2E
M_LOG10E M_LN2 M_LN10 M_PI M_PI_2 M_PI_4 M_1_PI M_2_PI M_2_SQRTPI M_SQRT2 M_SQRT1_2 min max GetObject`) {
		if !cppMacros[want] {
			t.Errorf("cppMacroNames lacks %s", want)
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
