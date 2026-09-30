package edit_test

import (
	"errors"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/project"
)

// uncDir is a project directory on a network share: its names carry a volume no walk up, join or clean may drop.
const (
	uncShare = "//server/share/"
	uncDir   = uncShare + "p"
)

// onVolumes skips a test on a host whose names have no volume: there "//server/share" is a path.
func onVolumes(t *testing.T) {
	t.Helper()
	if project.HostPaths().Volume(uncDir) == "" {
		t.Skip("the host's names carry no volume")
	}
}

// onShare is d with its project directory moved from projectDir to uncDir.
func onShare(d *diskFS) *diskFS {
	moved := newDiskFS(nil)
	rebase := func(name string) string {
		if rest, ok := strings.CutPrefix(name, projectDir); ok {
			return uncDir + rest
		}
		return name
	}
	for _, name := range slices.Sorted(maps.Keys(d.dirs)) {
		moved.mkdirs(rebase(name))
	}
	for _, name := range slices.Sorted(maps.Keys(d.files)) {
		moved.files[rebase(name)] = d.files[name]
	}
	for _, name := range slices.Sorted(maps.Keys(d.modes)) {
		moved.modes[rebase(name)] = d.modes[name]
	}
	return moved
}

// shareSite is fsys committed under uncDir by process 7 of host-a, which is dead.
func shareSite(fsys *faultFS) edit.Site {
	s := siteOf(fsys)
	s.Layout = layout{dir: uncDir}
	return s
}

// strays are the names of d outside the share, but the root the FS is seeded with.
func strays(d *diskFS) []string {
	var out []string
	for _, name := range slices.Sorted(maps.Keys(d.dirs)) {
		if !strings.HasPrefix(name, uncShare) && name != "/" {
			out = append(out, name)
		}
	}
	for _, name := range slices.Sorted(maps.Keys(d.files)) {
		if !strings.HasPrefix(name, uncShare) {
			out = append(out, name)
		}
	}
	return out
}

// shareFixture is commitFixture on the share.
func shareFixture() (*diskFS, []edit.Change) {
	d, changes := commitFixture()
	return onShare(d), changes
}

// afterCommit is commitFixture's file system once its edit is committed.
func afterCommit() *diskFS {
	d, changes := commitFixture()
	if err := commitIn(d, changes); err != nil {
		panic(err)
	}
	return d
}

// API.md N6, N8, N10 on a UNC project directory: every kind of change lands beside project.canon on the share, with the journal and the stages gone, and no name outside the share.
func TestCommitOnUNCVolume(t *testing.T) {
	onVolumes(t)
	d, changes := shareFixture()
	if err := edit.Commit(t.Context(), shareSite(&faultFS{diskFS: d}), testRev, changes); err != nil {
		t.Fatal(err)
	}
	mustState(t, d.state(), onShare(afterCommit()).state())
	if s := strays(d); len(s) != 0 {
		t.Errorf("names outside the share: %v", s)
	}
}

// API.md N11, O5 on a UNC project directory: a rename that fails is rolled back, and a commit whose process dies after any rename is put back by Recover, one Warn line.
func TestCommitCrashOnUNCVolume(t *testing.T) {
	onVolumes(t)
	probe, changes := shareFixture()
	counter := &faultFS{diskFS: probe}
	_ = edit.Commit(t.Context(), shareSite(counter), testRev, changes)
	if counter.renames < 4 {
		t.Fatalf("the fixture makes %d renames", counter.renames)
	}
	for k := 1; k <= counter.renames; k++ {
		d, changes := shareFixture()
		before := d.state()
		if err := edit.Commit(t.Context(), shareSite(&faultFS{diskFS: d, failRename: k}), testRev, changes); !errors.Is(err, errInjected) {
			t.Fatalf("rename %d: err = %v", k, err)
		}
		mustState(t, d.state(), before)
		d, changes = shareFixture()
		before = d.state()
		if err := edit.Commit(t.Context(), shareSite(&faultFS{diskFS: d, dieAfterRename: k}), testRev, changes); err == nil {
			t.Fatalf("rename %d: a dead commit returned nil", k)
		}
		lines, err := warningsAt(t, shareSite(&faultFS{diskFS: d}))
		if err != nil || len(lines) != 1 {
			t.Fatalf("rename %d: Recover logged %v, err %v", k, lines, err)
		}
		mustState(t, d.state(), before)
		if s := strays(d); len(s) != 0 {
			t.Errorf("rename %d: names outside the share: %v", k, s)
		}
	}
}
