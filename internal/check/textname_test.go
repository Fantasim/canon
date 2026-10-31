package check_test

import (
	"testing"

	"github.com/fantasim/canonlang/internal/check"
)

// CODEGEN.md §2.9, DECISIONS 418: a name whose extension is exactly .canon is no text file name.
func TestValidTextNameCanon(t *testing.T) {
	for _, tc := range []struct {
		name string
		want bool
	}{
		{"x.canon", false},
		{"a.b.canon", false},
		{"x.CANON", true},
		{"x.json", true},
		{"x.canon.txt", true},
	} {
		if got := check.ValidTextName(tc.name); got != tc.want {
			t.Errorf("ValidTextName(%q) = %v, want %v", tc.name, got, tc.want)
		}
	}
}
