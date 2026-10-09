package wire

import "testing"

// WIRE.md §5.4: a marker equals a value as JSON: a string however escaped, never another scalar of its text.
func TestSameJSONMarker(t *testing.T) {
	cases := []struct {
		raw, marker string
		want        bool
	}{
		{`"none"`, `"none"`, true},
		{`"no\u006ee"`, `"none"`, true},
		{`"other"`, `"none"`, false},
		{`1`, `"1"`, false},
		{`true`, `"true"`, false},
		{`null`, `"null"`, false},
		{`"1"`, `"1"`, true},
		{`-1`, `-1`, true},
		{`-1.0`, `-1`, true},
		{`-2`, `-1`, false},
		{`"-1"`, `-1`, false},
	}
	for _, c := range cases {
		if got := sameJSON(text(c.raw), []byte(c.marker)); got != c.want {
			t.Errorf("sameJSON(%s, %s) = %t, want %t", c.raw, c.marker, got, c.want)
		}
	}
}
