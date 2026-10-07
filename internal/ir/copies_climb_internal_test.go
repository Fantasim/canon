package ir

import "testing"

// TestClimbsAbove is CODEGEN.md §2.8 (DECISIONS 332): a relative path between two project-relative directories climbs above the project's parent directories when the part of the from-directory left after the common segments holds a `..`.
func TestClimbsAbove(t *testing.T) {
	for _, c := range []struct {
		from, to string
		want     bool
	}{
		{"../../../Source/Generated/x", "out/cpp", true},
		{"../../../Source/Generated/x", ".", true},
		{"../../../Source/Generated/x", "../../../Resource/y", false},
		{"../..", "../../sovcommon", false},
		{"../../sovcommon", "..", true},
		{"../a", "../../b", false},
		{"../../a", "../b", true},
		{"out/cpp", "../../../Source/Generated/x", false},
		{"out/cpp", "out/ts", false},
		{".", "out", false},
		{"out", ".", false},
	} {
		if got := climbsAbove(c.from, c.to); got != c.want {
			t.Errorf("climbsAbove(%q, %q) = %v, want %v", c.from, c.to, got, c.want)
		}
	}
}
