package edit_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// lastIndex is the last place of entry in log, -1 for none.
func lastIndex(log []string, entry string) int {
	for i := len(log) - 1; i >= 0; i-- {
		if log[i] == entry {
			return i
		}
	}
	return -1
}

// API.md N11 (log-2026-09-29 M4 U4c-r3): a rollback syncs every directory it touched after it
// put the files back and before it removes the journal.
func TestCommitRollbackSyncs(t *testing.T) {
	d, changes := commitFixture()
	before := d.state()
	s := newSyncFS(d)
	s.failRename = 2
	if err := commitIn(s, changes); !errors.Is(err, errInjected) {
		t.Fatalf("err = %v", err)
	}
	mustState(t, d.state(), before)
	restored, gone := lastIndex(s.log, opRename+" /p/a.canon"), lastIndex(s.log, opRemove+" "+journalFile)
	synced := slices.Index(s.log[restored:], opSync+" /p")
	if restored < 0 || synced < 0 || restored+synced > gone {
		t.Fatalf("log %v", s.log)
	}
}

// API.md N11 (log-2026-09-29 M4 U4c-r3): a rollback clears only the directories its own staging
// made: one someone else made meanwhile, with their file in it, stays.
func TestCommitRollbackOwnDirs(t *testing.T) {
	d, changes := commitFixture()
	f := &faultFS{diskFS: d, failRename: 1, onOp: func(op, name string, _ []byte) {
		if op == opWrite && name == journalFile {
			d.mu.Lock()
			d.mkdirs("/p/items/deep")
			d.files["/p/items/deep/theirs.canon"] = []byte("theirs\n")
			d.mu.Unlock()
		}
	}}
	if err := commitIn(f, changes); !errors.Is(err, errInjected) || errors.Is(err, edit.ErrJournal) {
		t.Fatalf("err = %v", err)
	}
	if string(d.files["/p/items/deep/theirs.canon"]) != "theirs\n" || d.dirs["/p/items/deep/x"] || len(journalsIn(d)) != 0 {
		t.Fatalf("theirs %q, x %v, journals %v", d.files["/p/items/deep/theirs.canon"], d.dirs["/p/items/deep/x"], journalsIn(d))
	}
}

// API.md N10, N11 (log-2026-09-29 M4 U4c-r3): a sync that fails fails the commit, which rolls
// back; a sync that keeps failing keeps the journal too, and Recover puts everything back.
func TestCommitSyncFails(t *testing.T) {
	cases := []struct {
		name  string
		dir   string
		times int
		kept  bool
	}{
		{"journal directory once", "/p/.canon/journal", 1, false},
		{"target directory once", "/p/items", 1, false},
		{"target directory always", "/p/items", -1, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, changes := commitFixture()
			before := d.state()
			s := newSyncFS(d)
			s.failDir, s.failTimes = tc.dir, tc.times
			if err := commitIn(s, changes); !errors.Is(err, errInjected) {
				t.Fatalf("err = %v", err)
			}
			if kept := len(journalsIn(d)) == 1; kept != tc.kept {
				t.Fatalf("journal kept %v", kept)
			}
			if !tc.kept {
				mustState(t, d.state(), before)
				return
			}
			if err := edit.Recover(siteOf(d), nil); err != nil {
				t.Fatal(err)
			}
			mustState(t, d.state(), before)
		})
	}
}
