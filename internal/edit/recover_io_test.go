package edit_test

import (
	"errors"
	"io/fs"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// stageSuffix ends every stage an edit writes (DECISIONS 351).
const stageSuffix = ".canon-edit"

// denyStagesFS is a faultFS whose stage writes fail once armed (nil: always) with the error the OS
// WriteFile gives for its temporary file, an absolute name the user never sees (DECISIONS 416).
type denyStagesFS struct {
	*faultFS
	armed func() bool
}

func (d denyStagesFS) WriteFile(name string, data []byte) error {
	if strings.HasSuffix(name, stageSuffix) && (d.armed == nil || d.armed()) {
		return &fs.PathError{Op: "open", Path: name + ".tmp", Err: fs.ErrPermission}
	}
	return d.faultFS.WriteFile(name, data)
}

// renamed is how many renames f has made or failed so far.
func (f *faultFS) renamed() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.renames
}

// API.md O5, N11 (DECISIONS 416): a rollback the file system refuses is a journal error, the
// journal kept; the OS error stays reachable, and the message names the display path only.
func TestRollbackIOErrorIsJournal(t *testing.T) {
	for _, c := range []struct {
		name string
		run  func(t *testing.T) (*diskFS, error)
	}{
		{"Recover", func(t *testing.T) (*diskFS, error) {
			d, _ := crashed(t)
			return d, edit.Recover(siteOf(denyStagesFS{faultFS: &faultFS{diskFS: d}}), nil)
		}},
		{"Settle", func(t *testing.T) (*diskFS, error) {
			d, _ := crashedHolding(t, "a")
			_, err := edit.Settle(lockSite(denyStagesFS{faultFS: &faultFS{diskFS: d}}, newLocker(), "a"), nil)
			return d, err
		}},
		{"commit rollback", func(t *testing.T) (*diskFS, error) {
			d, changes := commitFixture()
			f := &faultFS{diskFS: d, failRename: 2}
			return d, commitIn(denyStagesFS{faultFS: f, armed: func() bool { return f.renamed() >= 2 }}, changes)
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			d, err := c.run(t)
			checkRollbackIO(t, d, err)
		})
	}
}

// checkRollbackIO is DECISIONS 416's verdict on a refused rollback of a.canon.
func checkRollbackIO(t *testing.T, d *diskFS, err error) {
	t.Helper()
	var pe *fs.PathError
	switch {
	case !errors.Is(err, edit.ErrJournal):
		t.Errorf("err %v, want ErrJournal", err)
	case !errors.Is(err, fs.ErrPermission) || !errors.As(err, &pe):
		t.Errorf("err %v lost its *fs.PathError or fs.ErrPermission", err)
	case !strings.Contains(err.Error(), "a.canon") || strings.Contains(err.Error(), projectDir+"/"):
		t.Errorf("err %q, want a.canon named by its display path, no %s/", err, projectDir)
	}
	if len(journalsIn(d)) != 1 {
		t.Errorf("journals %v, want the journal kept", journalsIn(d))
	}
}
