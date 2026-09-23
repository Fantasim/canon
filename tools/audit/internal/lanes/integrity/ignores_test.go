package integrity

import "testing"

func TestCommentPart(t *testing.T) {
	tests := map[string]string{
		`const markNolint = "nolint"`:                  "",
		`x() //nolint:errcheck // best-effort cleanup`: "//nolint:errcheck // best-effort cleanup",
		` * the block comment's middle line`:           ` * the block comment's middle line`,
		`<!-- sovaudit:ignore dead-link -- a demo -->`: `<!-- sovaudit:ignore dead-link -- a demo -->`,
	}
	for in, want := range tests {
		if got := commentPart(in); got != want {
			t.Errorf("commentPart(%q) = %q, want %q", in, got, want)
		}
	}
}
