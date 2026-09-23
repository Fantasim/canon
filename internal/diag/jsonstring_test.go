package diag_test

import (
	"errors"
	"testing"
	"unicode/utf8"

	"github.com/fantasim/canonlang/internal/diag"
)

// WIRE.md §7.3: strings are written as ECMAScript JSON.stringify writes them.
func TestAppendJSONString(t *testing.T) {
	cases := []struct{ in, want string }{
		{"a\té/\"\x1b", `"a\té/\"\u001b"`},
		{"\b\f\n\r\\\x00\x7f", `"\b\f\n\r\\\u0000` + "\x7f\""},
		{"<>&\u2028\u2029", "\"<>&\u2028\u2029\""},
		{"bad\xffbyte", "\"bad\uFFFDbyte\""},
		{"", `""`},
	}
	for _, c := range cases {
		if got := string(diag.AppendJSONString(nil, c.in)); got != c.want {
			t.Errorf("AppendJSONString(%q) = %s, want %s", c.in, got, c.want)
		}
	}
}

// WIRE.md §3.2: escapes decoded; surrogates, controls, bad UTF-8 and no end refused.
func TestUnquoteJSON(t *testing.T) {
	valid := []struct {
		in, want string
		n        int
	}{
		{`"a"rest`, "a", 3},
		{`"\u0061\/\"\\\b\f\n\r\t"`, "a/\"\\\b\f\n\r\t", 24},
		{`"\ud83d\ude00"`, "\U0001F600", 14},
		{`"\u0000é"`, "\x00é", 10},
		{`"x y:z"`, "x y:z", 7},
	}
	for _, c := range valid {
		got, n, err := diag.UnquoteJSON(c.in)
		if err != nil || got != c.want || n != c.n {
			t.Errorf("UnquoteJSON(%q) = %q, %d, %v; want %q, %d", c.in, got, n, err, c.want, c.n)
		}
	}
	invalid := []string{``, `a`, `"abc`, `"\x"`, `"\u12"`, `"\u12G4"`, `"\ud800"`, `"\ud800\u0041"`,
		`"\udc00"`, `"\ud800x"`, "\"tab\there\"", "\"\xff\"", `"\`, `"\u+123"`}
	for _, in := range invalid {
		if _, _, err := diag.UnquoteJSON(in); !errors.Is(err, diag.ErrJSONString) {
			t.Errorf("UnquoteJSON(%q): got %v, want ErrJSONString", in, err)
		}
	}
}

// WIRE.md §3.2, §7.3: a string read back from its written form is itself.
func FuzzUnquoteJSON(f *testing.F) {
	f.Add(`"a\u00e9\ud83d\ude00"`)
	f.Add(`"\ud800"`)
	f.Fuzz(func(t *testing.T, in string) {
		v, n, err := diag.UnquoteJSON(in)
		if err != nil {
			return
		}
		if n > len(in) || !utf8.ValidString(v) {
			t.Fatalf("UnquoteJSON(%q) = %q, %d", in, v, n)
		}
		back, m, err := diag.UnquoteJSON(string(diag.AppendJSONString(nil, v)))
		if err != nil || back != v || m != len(diag.AppendJSONString(nil, v)) {
			t.Fatalf("%q: written %s, read back %q, %v", v, diag.AppendJSONString(nil, v), back, err)
		}
	})
}
