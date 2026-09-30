package workspace

import (
	"errors"
	"io/fs"
	"path/filepath"
	"slices"
	"testing"

	"github.com/fsnotify/fsnotify"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/project"
)

const (
	shareRoot = "//server/share/law"
	shareTop  = "//server/share/"
)

// hostVolumes skips a test on a host whose names have no volume: there "//server/share" is a path.
func hostVolumes(t *testing.T) {
	t.Helper()
	if project.HostPaths().Volume(shareRoot) == "" {
		t.Skip("the host's names carry no volume")
	}
}

// API.md N10, S9 on a UNC project directory: the journal's directory is the one under the share's project directory, so a commit's journal writes go to the disk unpinned.
func TestThroughJournalOnUNCVolume(t *testing.T) {
	hostVolumes(t)
	disk := &writeMapFS{}
	w := through(&snapFS{root: shareRoot}, disk)
	if got := w.to(shareRoot + "/.canon/journal/j.json"); got != build.WriteFS(disk) {
		t.Errorf("the journal is written through %T, not the disk", got)
	}
	if got := w.to(shareRoot + "/a/a.canon"); got == build.WriteFS(disk) {
		t.Error("a source is written to the disk, unpinned")
	}
}

// API.md W12 on a UNC project directory: a missing directory is watched by the nearest one above it that exists, never above the share.
func TestHubAddOnUNCVolume(t *testing.T) {
	hostVolumes(t)
	cases := []struct {
		name   string
		exists string
		want   []string
		gotDir string
		err    error
	}{
		{"below the project", shareRoot, []string{shareRoot + "/x/y", shareRoot + "/x", shareRoot}, shareRoot, nil},
		{"nothing exists", "", []string{shareRoot + "/x/y", shareRoot + "/x", shareRoot, shareTop}, shareTop, fs.ErrNotExist},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			n := &missingNotifier{exists: tc.exists}
			h := &hub{os: n, dirs: map[string]bool{}}
			got, added, err := h.add(shareRoot + "/x/y")
			if got != tc.gotDir || added != (tc.err == nil) || !errors.Is(err, tc.err) {
				t.Errorf("add = %q, %v, %v; want %q, %v", got, added, err, tc.gotDir, tc.err)
			}
			want := make([]string, len(tc.want))
			for i, d := range tc.want {
				want[i] = filepath.FromSlash(d)
			}
			if !slices.Equal(n.tried, want) {
				t.Errorf("watches tried %v, want %v", n.tried, want)
			}
		})
	}
}

// missingNotifier watches one directory only, and records every name it was asked to watch.
type missingNotifier struct {
	exists string
	tried  []string
}

func (m *missingNotifier) add(name string) error {
	m.tried = append(m.tried, name)
	if name != filepath.FromSlash(m.exists) {
		return &fs.PathError{Op: "watch", Path: name, Err: fs.ErrNotExist}
	}
	return nil
}

func (m *missingNotifier) remove(string) error           { return nil }
func (m *missingNotifier) close() error                  { return nil }
func (m *missingNotifier) events() <-chan fsnotify.Event { return nil }
func (m *missingNotifier) errs() <-chan error            { return nil }

// API.md W12 on a UNC project directory: each entry's directory is watched, by its own name.
func TestWatchDirsOnUNCVolume(t *testing.T) {
	hostVolumes(t)
	s := &snapFS{root: shareRoot, ents: map[name]*entry{
		{kind: kindFile, abs: shareRoot + "/a/a.canon"}: {},
		{kind: kindDir, abs: shareRoot + "/b"}:          {},
	}}
	want := []string{shareRoot, shareRoot + "/a", shareRoot + "/b"}
	if got := s.watchDirs(); !slices.Equal(got, want) {
		t.Errorf("watchDirs = %v, want %v", got, want)
	}
}
