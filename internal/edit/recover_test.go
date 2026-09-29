package edit_test

import (
	"bytes"
	"errors"
	"log/slog"
	"path"
	"reflect"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// warnings runs Recover over d and returns the lines it logged, all at Warn, and its error.
func warnings(t *testing.T, d *diskFS) ([]string, error) {
	t.Helper()
	return warningsAt(t, siteOf(d))
}

func warningsAt(t *testing.T, s edit.Site) ([]string, error) {
	t.Helper()
	var buf bytes.Buffer
	err := edit.Recover(s, slog.New(slog.NewTextHandler(&buf, nil)))
	if buf.Len() == 0 {
		return nil, err
	}
	lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	for _, l := range lines {
		if !strings.Contains(l, "level=WARN") {
			t.Fatalf("logged %q, not at Warn", l)
		}
	}
	return lines, err
}

// journalsIn are the files of the journal directory.
func journalsIn(d *diskFS) []string {
	entries, _ := d.ReadDir(path.Dir(journalFile))
	var out []string
	for _, e := range entries {
		if !e.IsDir() {
			out = append(out, e.Name())
		}
	}
	return out
}

// crashed is the fixture with its commit dead after the second rename: a journal and half an edit.
func crashed(t *testing.T) (d *diskFS, before string) {
	t.Helper()
	d, changes := commitFixture()
	before = d.state()
	if err := commitIn(&faultFS{diskFS: d, dieAfterRename: 2}, changes); !errors.Is(err, errDead) {
		t.Fatalf("err = %v", err)
	}
	if d.state() == before || len(journalsIn(d)) != 1 {
		t.Fatal("the crash left no half edit")
	}
	return d, before
}

// API.md O5: one Warn line, saying how many journals and changes were rolled back.
func TestRecoverLogsOneWarn(t *testing.T) {
	d, before := crashed(t)
	lines, err := warnings(t, d)
	if err != nil || len(lines) != 1 {
		t.Fatalf("logged %v, err %v", lines, err)
	}
	if !strings.Contains(lines[0], "journals=1") || !strings.Contains(lines[0], "changes=4") {
		t.Fatalf("logged %q", lines[0])
	}
	mustState(t, d.state(), before)
}

// API.md O5: recovering twice is recovering once; the second finds nothing and logs nothing.
func TestRecoverIdempotent(t *testing.T) {
	d, before := crashed(t)
	if _, err := warnings(t, d); err != nil {
		t.Fatal(err)
	}
	lines, err := warnings(t, d)
	if err != nil || len(lines) != 0 {
		t.Fatalf("second Recover logged %v, err %v", lines, err)
	}
	mustState(t, d.state(), before)
}

// API.md O5 (log-2026-09-29 M4 U5b): a journal whose files are all as they were is removed with
// the one Warn line of an unfinished edit rolled back, no file changed.
func TestRecoverUnchanged(t *testing.T) {
	d, changes := commitFixture()
	before := d.state()
	if err := commitIn(&faultFS{diskFS: d, dieAtOp: 3}, changes); !errors.Is(err, errDead) {
		t.Fatalf("err = %v", err)
	}
	if len(journalsIn(d)) != 1 {
		t.Fatal("no journal left")
	}
	lines, err := warnings(t, d)
	if err != nil || len(lines) != 1 || !strings.Contains(lines[0], "changes=0") || len(journalsIn(d)) != 0 {
		t.Fatalf("logged %v, err %v", lines, err)
	}
	mustState(t, d.state(), before)
}

// API.md O5: no journal directory, an empty one, or only files that are not journals: nothing
// to do, nothing logged, and those files stay.
func TestRecoverNothing(t *testing.T) {
	cases := []struct {
		name  string
		setup func(d *diskFS)
	}{
		{"no directory", func(*diskFS) {}},
		{"empty", func(d *diskFS) { d.mkdirs("/p/.canon/journal") }},
		{"other files", func(d *diskFS) {
			d.mkdirs("/p/.canon/journal")
			d.files["/p/.canon/journal/.ab.json.tmp"] = []byte("{")
			d.files["/p/.canon/journal/AB.json"] = []byte("{")
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, _ := commitFixture()
			tc.setup(d)
			before := d.state()
			lines, err := warnings(t, d)
			if err != nil || len(lines) != 0 {
				t.Fatalf("logged %v, err %v", lines, err)
			}
			mustState(t, d.state(), before)
		})
	}
}

// log-2026-09-29 M4 U4c-r, U4c-r3: the journal of a writer alive on this host is left alone,
// silently; a dead writer's is rolled back; another machine's is kept and refused with ErrJournal.
func TestRecoverRunningWriter(t *testing.T) {
	alive := func(pid int) bool { return pid == testPID }
	cases := []struct {
		name    string
		self    edit.Process
		applied bool
		refused bool
	}{
		{"alive here", edit.Process{Host: testHost, PID: 1, Alive: alive}, false, false},
		{"dead here", edit.Process{Host: testHost, PID: 1, Alive: func(int) bool { return false }}, true, false},
		{"other host", edit.Process{Host: "host-b", PID: 1, Alive: alive}, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, before := crashed(t)
			crash := d.state()
			s := siteOf(d)
			s.Self = tc.self
			lines, err := warningsAt(t, s)
			want := crash
			if tc.applied {
				want = before
			}
			if errors.Is(err, edit.ErrJournal) != tc.refused || !tc.refused && err != nil || (len(lines) == 1) != tc.applied {
				t.Fatalf("logged %v, err %v", lines, err)
			}
			mustState(t, d.state(), want)
		})
	}
}

// log-2026-09-29 M4 U4c-r: a file under a declared root outside the project is journaled by its
// absolute name and put back; a layout that no longer declares that root refuses the journal.
func TestRecoverRoot(t *testing.T) {
	d := newDiskFS(map[string]string{"/p/a.canon": "a old\n", "/data/x.json": "{}\n"})
	before := d.state()
	changes := []edit.Change{
		{Kind: edit.ChangeModified, Path: "@data/x.json", Before: []byte("{}\n"), After: []byte("{\"x\": 1}\n")},
		{Kind: edit.ChangeModified, Path: "a.canon", Before: []byte("a old\n"), After: []byte("a new\n")},
	}
	if err := commitIn(&faultFS{diskFS: d, dieAfterRename: 1}, changes); !errors.Is(err, errDead) {
		t.Fatalf("err = %v", err)
	}
	if !bytes.Contains(d.files[journalFile], []byte(`"path":"/data/x.json"`)) {
		t.Fatalf("journal %s", d.files[journalFile])
	}
	s := siteOf(d)
	s.Layout = layout{dir: projectDir}
	crash := d.state()
	if err := edit.Recover(s, nil); !errors.Is(err, edit.ErrJournal) {
		t.Fatalf("err = %v", err)
	}
	mustState(t, d.state(), crash)
	if err := edit.Recover(siteOf(d), nil); err != nil {
		t.Fatal(err)
	}
	mustState(t, d.state(), before)
}

// log-2026-09-29 M4 U4c-r, U4c-r3: a directory the edit created goes with the edit's new files,
// their stages and an FS's leftover named after a stage.
func TestRecoverLeftovers(t *testing.T) {
	d, changes := commitFixture()
	before := d.state()
	if err := commitIn(&faultFS{diskFS: d, dieAfterRename: 3}, changes); !errors.Is(err, errDead) {
		t.Fatalf("err = %v", err)
	}
	d.files["/p/items/deep/x/..c.canon.canon-edit.canon-tmp"] = []byte("half")
	d.files["/p/items/deep/x/.c.canon.canon-edit"] = []byte("c\n")
	if _, err := warnings(t, d); err != nil {
		t.Fatal(err)
	}
	mustState(t, d.state(), before)
}

// log-2026-09-29 M4 U4c-r3: a legitimate crash, then a file the user adds in a directory the
// edit created: the journal is refused whole, kept, and the user's file survives.
func TestRecoverUserFileInCreatedDir(t *testing.T) {
	d, changes := commitFixture()
	if err := commitIn(&faultFS{diskFS: d, dieAfterRename: 3}, changes); !errors.Is(err, errDead) {
		t.Fatalf("err = %v", err)
	}
	d.files["/p/items/deep/mine.canon"] = []byte("mine\n")
	crash := d.state()
	lines, err := warnings(t, d)
	if !errors.Is(err, edit.ErrJournal) || !strings.Contains(err.Error(), "/p/items/deep/mine.canon") || len(lines) != 0 {
		t.Fatalf("logged %v, err %v", lines, err)
	}
	mustState(t, d.state(), crash)
}

// API.md O5: a nil logger logs nothing and recovers all the same.
func TestRecoverNilLogger(t *testing.T) {
	d, before := crashed(t)
	if err := edit.Recover(siteOf(d), nil); err != nil {
		t.Fatal(err)
	}
	mustState(t, d.state(), before)
	if got := journalsIn(d); !reflect.DeepEqual(got, []string(nil)) {
		t.Fatalf("journals left: %v", got)
	}
}

// log-2026-09-29 M4 U4c-r: Recover syncs every directory it touched before it removes the journal.
func TestRecoverSync(t *testing.T) {
	d, _ := crashed(t)
	s := newSyncFS(d)
	if err := edit.Recover(siteOf(s), nil); err != nil {
		t.Fatal(err)
	}
	synced, gone := indexOf(s.log, opSync+" /p"), indexOf(s.log, opRemove+" "+journalFile)
	if synced < 0 || synced > gone {
		t.Fatalf("log %v", s.log)
	}
}
