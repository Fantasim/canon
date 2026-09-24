package check

import "testing"

// DECISIONS 153: the E2102 hint is the nearest other name, at distance 1 or 2 and below the
// typed name's length, ties by byte order; the name itself (distance 0) is never a hint.
func TestClosestHint(t *testing.T) {
	names := map[string]*object{}
	for _, n := range []string{"limit", "limits", "lamit", "ab"} {
		names[n] = &object{name: n}
	}
	c := &checker{universe: map[string]*object{}}
	env := &env{pkg: &pkgState{names: names}}
	for _, tc := range []struct{ typed, want string }{
		{"limit", "lamit"},
		{"limt", "limit"},
		{"xy", ""},
		{"a", ""},
		{"zzzzzz", ""},
	} {
		if got := c.closest(env, tc.typed); got != tc.want {
			t.Errorf("closest(%q) = %q, want %q", tc.typed, got, tc.want)
		}
	}
}
