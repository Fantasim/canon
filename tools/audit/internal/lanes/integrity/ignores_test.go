package integrity

import (
	"path/filepath"
	"testing"

	"github.com/fantasim/canonlang/tools/audit/internal/gosrc"
	"github.com/fantasim/canonlang/tools/audit/internal/lane"
	"github.com/fantasim/canonlang/tools/audit/internal/repo"
)

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

func TestIgnoreCountsEveryTool(t *testing.T) {
	dir := filepath.Join("testdata", "ignores")
	const rel = "markers.go"
	tree, perrs := gosrc.Parse(dir, []string{rel})
	if len(perrs) > 0 {
		t.Fatal(perrs)
	}
	ctx := &lane.Context{Repo: &repo.Repo{Root: dir}, Go: tree}
	directives, n := fileIgnores(ctx, rel)
	if len(directives) != 1 || n != 8 {
		t.Fatalf("fileIgnores = %d directives, %d ignores; want 1 and 8 (one per marker, none from the string)", len(directives), n)
	}
}
