package edit_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// counts commits the fixture through a faultFS breaking nothing but failRename, and returns
// how many renames and writes it made.
func counts(t *testing.T, failRename int) (renames, ops int) {
	t.Helper()
	d, changes := commitFixture()
	f := &faultFS{diskFS: d, failRename: failRename}
	_ = commitIn(f, changes)
	return f.renames, f.ops
}

// M4 acceptance 5, API.md N11: a rename that fails, whichever it is, leaves every file as it
// was before the edit, with no stage and no journal, and nothing for Recover to do.
func TestCommitCrashFailedRename(t *testing.T) {
	renames, _ := counts(t, 0)
	if renames < 4 {
		t.Fatalf("the fixture makes %d renames", renames)
	}
	for k := 1; k <= renames; k++ {
		d, changes := commitFixture()
		before := d.state()
		if err := commitIn(&faultFS{diskFS: d, failRename: k}, changes); !errors.Is(err, errInjected) {
			t.Fatalf("rename %d: err = %v", k, err)
		}
		mustState(t, d.state(), before)
		if lines, err := warnings(t, d); err != nil || len(lines) != 0 {
			t.Fatalf("rename %d: Recover logged %v, err %v", k, lines, err)
		}
	}
}

// M4 acceptance 5, API.md O5, N11: the process dies right after a rename, whichever it is;
// the next Open's Recover puts every file back and logs one Warn line.
func TestCommitCrashDeathAfterRename(t *testing.T) {
	renames, _ := counts(t, 0)
	for k := 1; k <= renames; k++ {
		d, changes := commitFixture()
		before := d.state()
		if err := commitIn(&faultFS{diskFS: d, dieAfterRename: k}, changes); err == nil {
			t.Fatalf("rename %d: a dead commit returned nil", k)
		}
		lines, err := warnings(t, d)
		if err != nil || len(lines) != 1 {
			t.Fatalf("rename %d: Recover logged %v, err %v", k, lines, err)
		}
		mustState(t, d.state(), before)
	}
}

// API.md O5, N6, N10, N11: the process dies at any write of a commit or of a failed rename's
// rollback, a WriteFile it dies in half written; Recover puts everything back, with one Warn line
// exactly when it removed a whole journal, a torn one silently (log-2026-09-29 M4 U5b).
func TestCommitCrashDeathAnywhere(t *testing.T) {
	renames, _ := counts(t, 0)
	for fail := 0; fail <= renames; fail++ {
		_, ops := counts(t, fail)
		for die := 1; die <= ops; die++ {
			d, changes := commitFixture()
			before := d.state()
			if err := commitIn(&faultFS{diskFS: d, failRename: fail, dieAtOp: die}, changes); err == nil {
				t.Fatalf("fail %d die %d: a dead commit returned nil", fail, die)
			}
			dirty, whole := d.settled() != before, json.Valid(d.files[journalFile])
			lines, err := warnings(t, d)
			if err != nil || (len(lines) == 1) != whole || len(lines) > 1 || dirty && !whole {
				t.Fatalf("fail %d die %d: dirty %v, journal whole %v, Recover logged %v, err %v", fail, die, dirty, whole, lines, err)
			}
			mustState(t, d.state(), before)
		}
	}
}

// API.md N11, N6: any write of a commit that fails, whichever it is, a directory's removal
// and the journal's included, leaves every file and directory as before, and no journal.
func TestCommitCrashFailedWrite(t *testing.T) {
	_, ops := counts(t, 0)
	for k := 1; k <= ops; k++ {
		d, changes := commitFixture()
		before := d.state()
		if err := commitIn(&faultFS{diskFS: d, failOp: k}, changes); !errors.Is(err, errInjected) {
			t.Fatalf("write %d: err = %v", k, err)
		}
		mustState(t, d.state(), before)
		if n := len(journalsIn(d)); n != 0 {
			t.Fatalf("write %d: %d journals left", k, n)
		}
	}
}

// maxRecoverWrites bounds the search for the writes a Recover makes.
const maxRecoverWrites = 1000

// API.md O5: the process dies again at any write of Recover itself, after a commit died after
// any rename; the next Recover still puts everything back.
func TestCommitCrashDeathDuringRecover(t *testing.T) {
	renames, _ := counts(t, 0)
	for k := 1; k <= renames; k++ {
		for die := 1; die < maxRecoverWrites; die++ {
			d, changes := commitFixture()
			before := d.state()
			_ = commitIn(&faultFS{diskFS: d, dieAfterRename: k}, changes)
			dying := &faultFS{diskFS: d, dieAtOp: die}
			_ = edit.Recover(siteOf(dying), nil)
			if err := edit.Recover(siteOf(d), nil); err != nil {
				t.Fatalf("rename %d, recover write %d: %v", k, die, err)
			}
			mustState(t, d.state(), before)
			if !dying.isDead() {
				break
			}
		}
	}
}
