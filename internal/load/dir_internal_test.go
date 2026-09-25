package load

import (
	"errors"
	"fmt"
	"io/fs"
	"testing"

	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
)

// ERRORS.md §1.4: causeOf's fixed Kind for err, chosen by errors.Is, never the OS message or a path.
func TestCauseOf(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want diag.Kind
	}{
		{"missing", fs.ErrNotExist, diag.KindReadMissing},
		{"wrapped missing", fmt.Errorf("open: %w", fs.ErrNotExist), diag.KindReadMissing},
		{"link loop", project.ErrSymlinkLoop, diag.KindReadLinkLoop},
		{"permission", fs.ErrPermission, diag.KindReadPermission},
		{"not a directory", errNotDir, diag.KindReadNotDir},
		{"too large", source.ErrFileTooLarge, diag.KindReadTooLarge},
		{"anything else", errors.New("boom"), diag.KindReadUnreadable},
	}
	for _, c := range cases {
		if got := causeOf(c.err); got != c.want {
			t.Errorf("causeOf(%s) = %v, want %v", c.name, got, c.want)
		}
	}
}
