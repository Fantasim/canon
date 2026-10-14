package workspace

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"testing"
	"time"

	"github.com/fantasim/canonlang/internal/project"
)

const (
	stampOld      = "aaaa"
	stampNew      = "bbbb"
	stampTarget   = "target.canon"
	stampTickWait = 5 * time.Second
	stampLate     = time.Hour
	stampYear     = 2020
)

// stampPlatforms is the GOOS set whose stat carries a change time (API.md S1, DECISIONS 374); a
// test variable (the same list as replacePlatforms in api/refresh_test.go), so a broken build tag fails the test instead of skipping it.
var stampPlatforms = []string{"linux", "android", "darwin", "ios", "freebsd", "netbsd", "openbsd", "dragonfly", "illumos"}

func stampTime() time.Time { return time.Date(stampYear, time.January, 1, 0, 0, 0, 0, time.UTC) }

func stampMust(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// stampChtimes sets abs back to the year stampYear.
func stampChtimes(t *testing.T, abs string) {
	t.Helper()
	stampMust(t, os.Chtimes(abs, stampTime(), stampTime()))
}

// stampChange is the change time of abs.
func stampChange(t *testing.T, abs string) int64 {
	t.Helper()
	info, err := os.Stat(abs)
	stampMust(t, err)
	ctime, _, _ := sysStamp(info.Sys())
	return ctime
}

// stampTick moves the clock of the file system: it touches a probe until the probe's change time
// differs from abs's, so the timestamp granularity never matters.
func stampTick(t *testing.T, abs string) {
	t.Helper()
	probe := filepath.Join(t.TempDir(), "probe")
	stampMust(t, os.WriteFile(probe, nil, 0o644))
	want := stampChange(t, abs)
	deadline := time.Now().Add(stampTickWait)
	for stampChange(t, probe) == want {
		if time.Now().After(deadline) {
			t.Fatal("the change time never moved")
		}
		stampMust(t, os.Chtimes(probe, time.Now(), time.Now()))
	}
}

// API.md S1, DECISIONS 374: an entry is racy while it was read less than racyWindow after its
// modification time or its change time; a change time of 0 never makes it so.
func TestStampRacy(t *testing.T) {
	at := time.Unix(stampYear, 0)
	old := at.Add(-stampLate).UnixNano()
	recent := at.Add(-time.Second).UnixNano()
	rows := []struct {
		name  string
		stamp stamp
		want  bool
	}{
		{"both old", stamp{mtime: old, ctime: old}, false},
		{"recent mtime", stamp{mtime: recent, ctime: old}, true},
		{"recent ctime", stamp{mtime: old, ctime: recent}, true},
		{"no ctime", stamp{mtime: old}, false},
	}
	for _, row := range rows {
		if got := row.stamp.racy(at); got != row.want {
			t.Errorf("%s: racy = %v, want %v", row.name, got, row.want)
		}
	}
}

// stampRow is one way a name changes behind a project's back: its entry kind, the target's
// absolute name, and the change applied after the first read.
type stampRow struct {
	name  string
	kind  kind
	after string // the file's content once the change is applied
	setup func(t *testing.T, dir string) string
	apply func(t *testing.T, abs string)
}

func stampFile(t *testing.T, dir string, mode os.FileMode) string {
	t.Helper()
	abs := filepath.Join(dir, stampTarget)
	stampMust(t, os.WriteFile(abs, []byte(stampOld), 0o644))
	stampChtimes(t, abs)
	stampMust(t, os.Chmod(abs, mode))
	return abs
}

func stampPlain(t *testing.T, dir string) string { return stampFile(t, dir, 0o644) }

var stampRows = []stampRow{
	{"in place", kindFile, stampNew, stampPlain, func(t *testing.T, abs string) {
		stampMust(t, os.WriteFile(abs, []byte(stampNew), 0o644))
		stampChtimes(t, abs)
	}},
	{"renamed over", kindFile, stampNew, stampPlain, func(t *testing.T, abs string) {
		tmp := abs + ".new"
		stampMust(t, os.WriteFile(tmp, []byte(stampNew), 0o644))
		stampChtimes(t, tmp)
		stampMust(t, os.Rename(tmp, abs))
	}},
	{"listing", kindDir, "", func(t *testing.T, dir string) string {
		d := filepath.Join(dir, "d")
		stampMust(t, os.Mkdir(d, 0o755))
		stampMust(t, os.WriteFile(filepath.Join(d, "a.canon"), nil, 0o644))
		stampChtimes(t, d)
		return d
	}, func(t *testing.T, abs string) {
		stampMust(t, os.Rename(filepath.Join(abs, "a.canon"), filepath.Join(abs, "b.canon")))
		stampChtimes(t, abs)
	}},
	{"made readable", kindFile, stampOld, func(t *testing.T, dir string) string {
		if os.Geteuid() == 0 {
			t.Skip("root reads a file of mode 000")
		}
		return stampFile(t, dir, 0)
	}, func(t *testing.T, abs string) { stampMust(t, os.Chmod(abs, 0o644)) }},
}

// API.md S1, DECISIONS 374: a name replaced keeping its size and modification time, or made
// readable, is read again because its change time or identity moved.
func TestRefreshChangeTime(t *testing.T) {
	if !slices.Contains(stampPlatforms, runtime.GOOS) {
		t.Skip("the stat has no change time on " + runtime.GOOS)
	}
	for _, row := range stampRows {
		t.Run(row.name, func(t *testing.T) {
			dir, err := filepath.EvalSymlinks(t.TempDir())
			stampMust(t, err)
			abs := row.setup(t, dir)
			late := func() time.Time { return time.Now().Add(stampLate) }
			s := newSnapFS(project.OS(), nil, late)
			s.root = dir
			if row.kind == kindDir {
				_, _ = s.ReadDir(abs)
			} else {
				_, _ = s.ReadFile(abs)
			}
			stampTick(t, abs)
			row.apply(t, abs)
			next, changed, err := s.refresh(context.Background())
			stampMust(t, err)
			if want := []name{{kind: row.kind, abs: abs}}; !slices.Equal(changed, want) {
				t.Fatalf("changed = %v, want %v", changed, want)
			}
			if row.kind == kindFile {
				if got, err := next.ReadFile(abs); err != nil || string(got) != row.after {
					t.Fatalf("next reads %q, %v; want %q", got, err, row.after)
				}
			}
		})
	}
}
