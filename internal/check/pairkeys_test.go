package check

import (
	"strconv"
	"strings"
	"testing"
)

// WIRE.md §4.1, §5.14: the symbolic template checks agree with expanding every slot.
func TestPairTemplatesMatchExpansion(t *testing.T) {
	parts := []string{"", "a", "1", "0", "10", "01"}
	sizes := []int{1, 10, 11, 101}
	var ts []pairTemplate
	for i := range len(parts) * len(parts) * len(sizes) {
		pre, suf, n := parts[i%len(parts)], parts[i/len(parts)%len(parts)], sizes[i/len(parts)/len(parts)]
		ts = append(ts, pairTemplate{pre: pre, suf: suf, n: n})
	}
	for _, a := range ts {
		for _, b := range ts {
			agree(t, a, b)
		}
	}
}

func agree(t *testing.T, a, b pairTemplate) {
	t.Helper()
	keysB := map[string]bool{}
	for _, k := range expand(b) {
		keysB[k] = true
		if !b.matches(k) {
			t.Fatalf("%+v does not match its own key %q", b, k)
		}
	}
	want := false
	for _, k := range expand(a) {
		want = want || keysB[k]
	}
	k, got := commonKey(a, b)
	if got != want || (got && (!a.matches(k) || !b.matches(k))) {
		t.Fatalf("commonKey(%+v, %+v) = %q, %v; expansion says %v", a, b, k, got, want)
	}
}

func expand(t pairTemplate) []string {
	var out []string
	for i := range t.n {
		out = append(out, t.pre+strconv.Itoa(i)+t.suf)
	}
	return out
}

// WIRE.md §4.1: a slot's text is decimal without a leading zero, below the bound.
func TestSlotBelow(t *testing.T) {
	for _, tc := range []struct {
		d    string
		n    int
		want bool
	}{
		{"0", 1, true},
		{"1", 1, false},
		{"01", 5, false},
		{"", 5, false},
		{"+1", 5, false},
		{"999999999", 1_000_000_000, true},
		{"1000000000", 1_000_000_000, false},
		{strings.Repeat("9", 25), 5, false},
	} {
		if got := slotBelow(tc.d, tc.n); got != tc.want {
			t.Errorf("slotBelow(%q, %d) = %v, want %v", tc.d, tc.n, got, tc.want)
		}
	}
}
