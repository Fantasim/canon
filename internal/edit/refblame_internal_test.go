package edit

import "testing"

// API.md E12 (DECISIONS 464, 483): the reason a rename is refused with is its first, but
// `layered`, the more specific blame, wins over any other, `computed` included.
func TestRenameBlame(t *testing.T) {
	cases := []struct{ r, why, want Reason }{
		{ReasonNone, ReasonComputed, ReasonComputed},
		{ReasonComputed, ReasonLayer, ReasonComputed},
		{ReasonComputed, ReasonLayered, ReasonLayered},
		{ReasonLayered, ReasonComputed, ReasonLayered},
	}
	for _, c := range cases {
		if got := blame(c.r, c.why); got != c.want {
			t.Errorf("blame(%v, %v) = %v, want %v", c.r, c.why, got, c.want)
		}
	}
}
