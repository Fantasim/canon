package cli

import (
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
)

// CLI.md §2.3, §2.4: only a finding's header token is coloured, never a detail, related or message line.
func TestPaintHeadersOnly(t *testing.T) {
	e, w := "error["+string(diag.E3501.Def().Code)+"]", "warning["+string(diag.W1002.Def().Code)+"]"
	loc, mention := "  a.canon:1:1\n", "  see error[E1] here\n  error[E1]\n"
	related := "  first\nerror line two\n  expected by b.canon:2 (x)\n\n1 error, 0 warnings\n"
	cases := []struct{ name, in, want string }{
		{"error", e + loc, ansiRed + e + ansiReset + loc},
		{"warning", w + loc, ansiYellow + w + ansiReset + loc},
		{"message names a code", e + loc + mention, ansiRed + e + ansiReset + loc + mention},
		{"related and multi-line", e + loc + related, ansiRed + e + ansiReset + loc + related},
		{"summary", "2 errors, 1 warning in 1 package (1 ms)\n", "2 errors, 1 warning in 1 package (1 ms)\n"},
	}
	inv := &invocation{opt: newOptions()}
	inv.opt.color = colorAlways
	for _, c := range cases {
		if got := inv.paint(c.in); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
