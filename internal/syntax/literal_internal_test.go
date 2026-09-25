package syntax

import "testing"

// GRAMMAR.md §2.7, LEX-01.
func TestRegexPattern(t *testing.T) {
	cases := []struct {
		rule, body, want string
	}{
		{"LEX-01 escaped slash", `\/`, `/`},
		{"LEX-01 backslash-slash class", `[\\/]`, `[\\/]`},
		{"LEX-01 escaped backslash then escaped slash", `\\\/`, `\\/`},
		{"LEX-01 trailing backslash", `a\`, `a\`},
		{"LEX-01 other escapes pass through", `\d\/\d`, `\d/\d`},
		{"LEX-01 no escapes", `^[A-Z]+$`, `^[A-Z]+$`},
		{"LEX-01 empty body", ``, ``},
	}
	for _, c := range cases {
		if got := regexPattern([]byte(c.body)); got != c.want {
			t.Errorf("%s: regexPattern(%q) = %q, want %q", c.rule, c.body, got, c.want)
		}
	}
}
