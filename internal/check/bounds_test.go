package check_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
)

// TYPES.md §1, §15, DECISIONS 150, 210: one finding per bound, at what is not constant, none for a broken name.
func TestBoundFindings(t *testing.T) {
	e3015 := diag.E3015.Def().Code
	for _, tc := range []struct {
		name string
		src  string
		want []diag.Code
		at   string // where the constant finding is, when one is wanted
	}{
		{name: "§15 a call to a fn declared later, reported at the call",
			src:  "record R { x: Int(0..=1 + f()) }\nfn f() -> Int { return 1 }\n",
			want: []diag.Code{e3015}, at: "a/a.canon:3:27"},
		{name: "§15 a call to a fn declared earlier, reported at the call once",
			src:  "fn f() -> Int { return 1 }\nrecord R { x: Int(0..=1 + f()) }\n",
			want: []diag.Code{e3015}, at: "a/a.canon:4:27"},
		{name: "§1 a bound reading a broken const",
			src:  "const A = nope\nrecord R { x: Int(0..=A) }\n",
			want: []diag.Code{diag.E2102.Def().Code}},
		{name: "§1 a bound reading a const that reads a broken const",
			src:  "const B = nope\nconst A = B + 1\nrecord R { x: Int(0..=A) }\n",
			want: []diag.Code{diag.E2102.Def().Code}},
		{name: "§1 a bound already reported is not folded",
			src:  "record R { x: Int(0..=nope) }\n",
			want: []diag.Code{diag.E2102.Def().Code}},
		{name: "DECISIONS 214 a bound reading a const with a syntax error",
			src:  "const A = 1__0\nrecord R { x: Int(0..=A) }\nlet r: R = { x: 0 }\n",
			want: []diag.Code{diag.E1110.Def().Code}},
		{name: "DECISIONS 215 a bound literal with a lexer error is not folded",
			src:  "record R { x: Int(0..=0x) }\nlet r: [R] = [{ x: 5 }]\n",
			want: []diag.Code{diag.E1110.Def().Code}},
		{name: "DECISIONS 215 a bound literal with a misplaced separator is not folded",
			src:  "record R { x: Int(0..=1_) }\nlet r: [R] = [{ x: 5 }]\n",
			want: []diag.Code{diag.E1110.Def().Code}},
		{name: "DECISIONS 215 a parameter default with a lexer error is not checked against its range",
			src:  "fn f(n: Int(1..) = 0x) -> Int { return n }\n",
			want: []diag.Code{diag.E1110.Def().Code}},
		{name: "§15 a field default calling a fn declared later",
			src:  "record R { x: Int = f() }\nfn f() -> Int { return 1 }\n",
			want: []diag.Code{diag.E3010.Def().Code}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := wantErrors(t, tc.src, tc.want)
			if tc.at != "" && !strings.Contains(out, "error["+string(e3015)+"]  "+tc.at+"\n") {
				t.Errorf("want %s at %s:\n%s", e3015, tc.at, out)
			}
		})
	}
}

// wantErrors checks src, declarations of package a, alone and through a build: both report
// exactly the errors want. It returns check's rendered findings.
func wantErrors(t *testing.T, src string, want []diag.Code) string {
	t.Helper()
	text := "package " + builtPackage + "\n\n" + src
	b := checkBuilt(t, text)
	if w := sortedCodes(want); !slices.Equal(b.codes, w) {
		t.Errorf("codes %v, want %v:\n%s", b.codes, w, b.out)
	}
	if got, w := errorCodes(buildOne(t, text).List), sortedCodes(want); !slices.Equal(got, w) {
		t.Errorf("a build reports %v, want %v", got, w)
	}
	return b.out
}

// DECISIONS 215: a lexer error gives its literal token the error type, never the composite around it.
func TestLexErrorTypesTheTokenOnly(t *testing.T) {
	e1110, e1109, e2102 := diag.E1110.Def().Code, diag.E1109.Def().Code, diag.E2102.Def().Code
	for _, tc := range []struct {
		src  string
		want []diag.Code
	}{
		{src: "let x: [Int] = [0x, nope]\n", want: []diag.Code{e1110, e2102}},
		{src: "let c: Bool = true\nlet y: Int = if c { 0x } else { nope }\n", want: []diag.Code{e1110, e2102}},
		{src: "/// A pair.\nrecord P {\n  /// A.\n  a: Int\n  /// B.\n  b: Int\n}\nlet p: P = { a: 0x, b: nope }\n", want: []diag.Code{e1110, e2102}},
		{src: "let m: {String: Int} = { \"a\\q\": nope }\n", want: []diag.Code{e1109, e2102}},
		{src: "let s: String = \"x {0x} {nope}\"\n", want: []diag.Code{e1110, e2102}},
	} {
		t.Run(tc.src, func(t *testing.T) {
			wantErrors(t, tc.src, tc.want)
		})
	}
}
