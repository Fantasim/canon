package geninputschain_test

import (
	"testing"

	p "example.com/data/geninputschain/out/go"
)

// TestPatternChain is TYPES.md §7.4 and EVALUATION.md §11.3 step 3: Config.code is read only
// when every pattern of its chain matches (`^[a-z]+$` inner, `^.{1,4}$` outer); a failure is
// one line naming the variable, however many patterns fail.
func TestPatternChain(t *testing.T) {
	const refused = "GENINPUTSCHAIN_CODE: does not match its pattern"
	for _, c := range []struct{ text, want string }{
		{"abc", ""},
		{"abcd", ""},
		{"ABC", refused},    // passes the outer pattern, fails the inner one
		{"abcdef", refused}, // passes the inner pattern, fails the outer one
		{"ABCDEF", refused}, // fails both
	} {
		t.Setenv("GENINPUTSCHAIN_CODE", c.text)
		err := p.LoadInputs()
		got := ""
		if err != nil {
			got = err.Error()
		}
		if got != c.want {
			t.Errorf("%q: error %q, want %q", c.text, got, c.want)
		}
		v, ok := (&p.Config{}).Code()
		if ok != (c.want == "") || ok && v != c.text {
			t.Errorf("%q: Code() = %q, %v", c.text, v, ok)
		}
	}
}
