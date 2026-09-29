package edit_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"maps"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
)

const projectDir = "/p"

var (
	testRev     = "r1:" + strings.Repeat("ab", 32)
	journalFile = projectDir + "/.canon/journal/" + strings.Repeat("ab", 32) + ".json"
)

// commitFixture is a project and an edit of every kind over it: two modified files, one created
// in directories that do not exist, two deleted, one renamed, and three directories left empty.
func commitFixture() (*diskFS, []edit.Change) {
	d := newDiskFS(map[string]string{
		"/p/project.canon":           "project\n",
		"/p/a.canon":                 "a old\n",
		"/p/b.canon":                 "b old\n",
		"/p/gone.canon":              "gone\n",
		"/p/items/old.canon":         "old key\n",
		"/p/pkg/pkg.canon":           "package pkg\n",
		"/p/pkg/sub/leaf/only.canon": "only\n",
	})
	d.modes["/p/items/old.canon"] = 0o640
	d.mkdirs("/p/pkg/sub/void")
	d.modes["/p/pkg/sub/void"] = 0o700 // empty already: only the journal recreates it
	return d, []edit.Change{
		{Kind: edit.ChangeModified, Path: "a.canon", Before: []byte("a old\n"), After: []byte("a new\n")},
		{Kind: edit.ChangeModified, Path: "b.canon", Before: []byte("b old\n"), After: []byte("b new\n")},
		{Kind: edit.ChangeCreated, Path: "items/deep/x/c.canon", After: []byte("c\n")},
		{Kind: edit.ChangeDeleted, Path: "gone.canon", Before: []byte("gone\n")},
		{Kind: edit.ChangeRenamed, OldPath: "items/old.canon", Path: "items/new.canon", Before: []byte("old key\n"), After: []byte("new key\n")},
		{Kind: edit.ChangeDeleted, Path: "pkg/sub/leaf/only.canon", Before: []byte("only\n")},
		{Kind: edit.ChangeRemovedDir, Path: "pkg/sub/leaf"},
		{Kind: edit.ChangeRemovedDir, Path: "pkg/sub/void"},
		{Kind: edit.ChangeRemovedDir, Path: "pkg/sub"},
	}
}

// committed is the fixture after its edit: the renamed file keeps its mode, a new one has the FS's.
func committed() string {
	d := newDiskFS(map[string]string{
		"/p/project.canon":        "project\n",
		"/p/a.canon":              "a new\n",
		"/p/b.canon":              "b new\n",
		"/p/items/deep/x/c.canon": "c\n",
		"/p/items/new.canon":      "new key\n",
		"/p/pkg/pkg.canon":        "package pkg\n",
	})
	d.modes["/p/items/deep/x/c.canon"], d.modes["/p/items/new.canon"] = 0o600, 0o640
	return d.state()
}

func commitIn(fsys build.WriteFS, changes []edit.Change) error {
	return edit.Commit(context.Background(), siteOf(fsys), testRev, changes)
}

// API.md N6, N8, N10: every kind of change lands; stages and journal are gone; a modified or
// renamed file keeps its permissions.
func TestCommitWrites(t *testing.T) {
	d, changes := commitFixture()
	if err := commitIn(d, changes); err != nil {
		t.Fatal(err)
	}
	mustState(t, d.state(), committed())
}

// API.md N9: a file that differs on disk from the snapshot refuses the commit, which writes nothing.
func TestCommitStale(t *testing.T) {
	cases := []struct {
		name  string
		touch func(d *diskFS)
		want  []string
	}{
		{"modified", func(d *diskFS) { d.files["/p/b.canon"] = []byte("b other\n") }, []string{"b.canon"}},
		{"deleted meanwhile", func(d *diskFS) { delete(d.files, "/p/gone.canon") }, []string{"gone.canon"}},
		{"created meanwhile", func(d *diskFS) {
			d.mkdirs("/p/items/deep/x")
			d.files["/p/items/deep/x/c.canon"] = []byte("c\n")
		}, []string{"items/deep/x/c.canon"}},
		{"renamed source changed", func(d *diskFS) { d.files["/p/items/old.canon"] = []byte("x\n") }, []string{"items/old.canon"}},
		{"two", func(d *diskFS) {
			d.files["/p/b.canon"] = []byte("x\n")
			d.files["/p/a.canon"] = []byte("x\n")
		}, []string{"a.canon", "b.canon"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, changes := commitFixture()
			tc.touch(d)
			before := d.state()
			err := commitIn(d, changes)
			var stale *edit.StaleError
			if !errors.As(err, &stale) || !errors.Is(err, edit.ErrStale) || !reflect.DeepEqual(stale.Files, tc.want) {
				t.Fatalf("err = %v, want stale %v", err, tc.want)
			}
			mustState(t, d.state(), before)
			if d.dirs["/p/.canon"] {
				t.Fatal("a refused commit created the journal directory")
			}
		})
	}
}

// API.md N12: written files have '\n' line ends and exactly one final '\n'.
func TestCommitLineEnds(t *testing.T) {
	cases := []struct{ after, want string }{
		{"x", "x\n"},
		{"x\n", "x\n"},
		{"x\n\n\n", "x\n"},
		{"a\r\nb\r\n", "a\nb\n"},
		{"a\r\nb\r\n\r\n", "a\nb\n"},
		{"", "\n"},
	}
	for _, tc := range cases {
		d := newDiskFS(map[string]string{"/p/a.canon": "old\n"})
		changes := []edit.Change{{Kind: edit.ChangeModified, Path: "a.canon", Before: []byte("old\n"), After: []byte(tc.after)}}
		if err := commitIn(d, changes); err != nil {
			t.Fatal(err)
		}
		if got := string(d.files["/p/a.canon"]); got != tc.want {
			t.Errorf("%q written as %q, want %q", tc.after, got, tc.want)
		}
	}
}

// journalOf commits changes over d and returns the journal it wrote, with the writes before it.
func journalOf(t *testing.T, d *diskFS, changes []edit.Change) (data []byte, name string, before []string) {
	t.Helper()
	var sequence []string
	f := &faultFS{diskFS: d, onOp: func(op, n string, b []byte) {
		if strings.HasPrefix(n, projectDir+"/.canon/journal/") && op == opWrite {
			data, name, before = b, n, slices.Clone(sequence)
		}
		sequence = append(sequence, op)
	}}
	if err := commitIn(f, changes); err != nil {
		t.Fatal(err)
	}
	return data, name, before
}

type journalView struct {
	Revision string
	Host     string
	PID      int
	Files    []struct {
		Path    string
		Existed bool
		Old     []byte
		Mode    *uint32
		New     string
	}
	Created []string
	Removed []struct {
		Path string
		Mode *uint32
	}
}

func sum(text string) string {
	s := sha256.Sum256([]byte(text))
	return hex.EncodeToString(s[:])
}

// API.md N10 (log-2026-09-29 M4 U4c-r): the journal, named by the revision's hex, holds its writer,
// each file's old content, mode and new digest, the directories made and removed, in byte order,
// the same bytes whatever the change order; only its directories are made before it.
func TestCommitJournal(t *testing.T) {
	d, changes := commitFixture()
	data, name, seq := journalOf(t, d, changes)
	if name != journalFile || !reflect.DeepEqual(seq, []string{opMkdir}) {
		t.Fatalf("journal %q after %v", name, seq)
	}
	var j journalView
	if err := json.Unmarshal(data, &j); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	for _, f := range j.Files {
		mode := "-"
		if f.Mode != nil {
			mode = fs.FileMode(*f.Mode).String()
		}
		files[f.Path] = strings.Join([]string{string(f.Old), mode, f.New}, "|")
	}
	want := map[string]string{
		"a.canon": "a old\n|-rw-r--r--|" + sum("a new\n"), "b.canon": "b old\n|-rw-r--r--|" + sum("b new\n"),
		"gone.canon": "gone\n|-rw-r--r--|absent", "items/deep/x/c.canon": "|-|" + sum("c\n"),
		"items/new.canon": "|-|" + sum("new key\n"), "items/old.canon": "old key\n|-rw-r-----|absent",
		"pkg/sub/leaf/only.canon": "only\n|-rw-r--r--|absent",
	}
	if !reflect.DeepEqual(files, want) || j.Revision != testRev || j.Host != testHost || j.PID != testPID {
		t.Fatalf("journal %s", data)
	}
	var removed []string
	for _, r := range j.Removed {
		removed = append(removed, r.Path+" "+fs.FileMode(*r.Mode).String())
	}
	if !reflect.DeepEqual(j.Created, []string{"items/deep", "items/deep/x"}) ||
		!reflect.DeepEqual(removed, []string{"pkg/sub -rwxr-xr-x", "pkg/sub/leaf -rwxr-xr-x", "pkg/sub/void -rwx------"}) {
		t.Fatalf("journal %s", data)
	}
	d2, changes2 := commitFixture()
	slices.Reverse(changes2)
	if again, _, _ := journalOf(t, d2, changes2); !bytes.Equal(again, data) || data[len(data)-1] != '\n' {
		t.Fatalf("journal not deterministic:\n%s\n%s", data, again)
	}
}

// API.md S11: cancelled before the first rename, a commit writes nothing; after it, it completes.
func TestCommitCancel(t *testing.T) {
	cases := []struct {
		name     string
		at       string // the op, or the file written, that cancels; "" before the call
		wantErr  bool
		complete bool
	}{
		{"before the call", "", true, false},
		{"while staging", opChmod, true, false},
		{"at the last stage", "/p/items/.new.canon.canon-edit", true, false},
		{"at the first rename", opRename, false, true},
		{"at the last rename", "/p/items/new.canon", false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, changes := commitFixture()
			before := d.state()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			f := &faultFS{diskFS: d, onOp: func(op, name string, _ []byte) {
				if op == tc.at || name == tc.at {
					cancel()
				}
			}}
			if tc.at == "" {
				cancel()
			}
			err := edit.Commit(ctx, siteOf(f), testRev, changes)
			if tc.wantErr != errors.Is(err, context.Canceled) || !tc.wantErr && err != nil {
				t.Fatalf("err = %v", err)
			}
			want := before
			if tc.complete {
				want = committed()
			}
			mustState(t, d.state(), want)
		})
	}
}

// API.md N10: a revision that is not <scheme>:<lowercase hex>, a change of no known kind, a
// path with no place and two changes of one file are refused before anything is written.
func TestCommitRefused(t *testing.T) {
	mod := func(p string) edit.Change {
		return edit.Change{Kind: edit.ChangeModified, Path: p, Before: []byte("a old\n"), After: []byte("x")}
	}
	cases := []struct {
		name    string
		rev     string
		changes []edit.Change
		want    error
	}{
		{"no prefix", strings.Repeat("ab", 32), []edit.Change{mod("a.canon")}, edit.ErrRevision},
		{"empty hex", "r1:", []edit.Change{mod("a.canon")}, edit.ErrRevision},
		{"upper hex", "r1:AB", []edit.Change{mod("a.canon")}, edit.ErrRevision},
		{"no scheme", ":ab", []edit.Change{mod("a.canon")}, edit.ErrRevision},
		{"unknown kind", testRev, []edit.Change{{Kind: edit.ChangeRemovedDir + 1, Path: "a.canon"}}, edit.ErrChanges},
		{"no place", testRev, []edit.Change{mod("@nowhere/a.json")}, edit.ErrChanges},
		{"twice", testRev, []edit.Change{mod("a.canon"), mod("./a.canon")}, edit.ErrChanges},
		{"hidden", testRev, []edit.Change{{Kind: edit.ChangeCreated, Path: ".git/x.canon", After: []byte("x")}}, edit.ErrChanges},
		{"not written by edits", testRev, []edit.Change{{Kind: edit.ChangeCreated, Path: "x.sh", After: []byte("x")}}, edit.ErrChanges},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, _ := commitFixture()
			before := d.state()
			if err := edit.Commit(context.Background(), siteOf(d), tc.rev, tc.changes); !errors.Is(err, tc.want) {
				t.Fatalf("err = %v, want %v", err, tc.want)
			}
			mustState(t, d.state(), before)
		})
	}
}

// log-2026-09-29 M4 U4c-r: while any journal is there, even a stale edit is refused with
// ErrJournal, before its files are read, and nothing is written.
func TestCommitBusy(t *testing.T) {
	d, changes := commitFixture()
	d.mkdirs("/p/.canon/journal")
	d.files["/p/.canon/journal/"+strings.Repeat("cd", 32)+".json"] = []byte("{}")
	d.files["/p/b.canon"] = []byte("b other\n")
	before := d.state()
	if err := commitIn(d, changes); !errors.Is(err, edit.ErrJournal) || errors.Is(err, edit.ErrStale) {
		t.Fatalf("err = %v", err)
	}
	mustState(t, d.state(), before)
}

// API.md N6: a directory the edit leaves empty is removed after the files, deepest first in
// whatever order it came; one not empty at commit time stays, one already gone is no failure.
func TestCommitRemovedDir(t *testing.T) {
	gone := edit.Change{Kind: edit.ChangeDeleted, Path: "pkg/sub/leaf/only.canon", Before: []byte("only\n")}
	cases := []struct {
		name  string
		extra map[string]string
		dirs  []string
		want  []string // the directories under /p/pkg after the commit
	}{
		{"shallow first", nil, []string{"pkg/sub", "pkg/sub/leaf"}, nil},
		{"not empty", map[string]string{"/p/pkg/sub/other.canon": "x\n"}, []string{"pkg/sub/leaf", "pkg/sub"}, []string{"/p/pkg/sub"}},
		{"not there", nil, []string{"pkg/sub/leaf", "pkg/sub", "pkg/none"}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			files := map[string]string{"/p/pkg/pkg.canon": "package pkg\n", "/p/pkg/sub/leaf/only.canon": "only\n"}
			maps.Copy(files, tc.extra)
			d := newDiskFS(files)
			changes := []edit.Change{gone}
			for _, dir := range tc.dirs {
				changes = append(changes, edit.Change{Kind: edit.ChangeRemovedDir, Path: dir})
			}
			if err := commitIn(d, changes); err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, name := range slices.Sorted(maps.Keys(d.dirs)) {
				if strings.HasPrefix(name, "/p/pkg/") {
					got = append(got, name)
				}
			}
			if !reflect.DeepEqual(got, tc.want) || !d.dirs["/p/pkg"] || len(journalsIn(d)) != 0 {
				t.Fatalf("directories %v, want %v", got, tc.want)
			}
		})
	}
}

// API.md N11 (log-2026-09-29 M4 U4c-r): a rollback puts back only what the commit changed; a
// file it never touched, changed by someone else meanwhile, stays as they left it.
func TestCommitRollbackOwnFiles(t *testing.T) {
	d, changes := commitFixture()
	f := &faultFS{diskFS: d, failRename: 1, onOp: func(op, _ string, _ []byte) {
		if op == opRename {
			d.mu.Lock()
			d.files["/p/b.canon"] = []byte("b theirs\n")
			d.mu.Unlock()
		}
	}}
	err := commitIn(f, changes)
	if !errors.Is(err, errInjected) || errors.Is(err, edit.ErrJournal) {
		t.Fatalf("err = %v", err)
	}
	if got := string(d.files["/p/b.canon"]); got != "b theirs\n" || len(journalsIn(d)) != 0 {
		t.Fatalf("b.canon %q, journals %v", got, journalsIn(d))
	}
}

// API.md N11 (log-2026-09-29 M4 U4c-r): a file the commit changed that someone else changed
// again refuses the rollback whole; the journal stays, and so does Recover's refusal, until the
// file is back in a state the journal knows.
func TestCommitRollbackConflict(t *testing.T) {
	d, changes := commitFixture()
	f := &faultFS{diskFS: d, failRename: 2, onOp: func(op, name string, _ []byte) {
		if op == opRename && name == "/p/b.canon" {
			d.mu.Lock()
			d.files["/p/a.canon"] = []byte("a theirs\n")
			d.mu.Unlock()
		}
	}}
	if err := commitIn(f, changes); !errors.Is(err, errInjected) || !errors.Is(err, edit.ErrJournal) {
		t.Fatalf("err = %v", err)
	}
	if string(d.files["/p/b.canon"]) != "b old\n" || len(journalsIn(d)) != 1 {
		t.Fatalf("b.canon %q, journals %v", d.files["/p/b.canon"], journalsIn(d))
	}
	if lines, err := warnings(t, d); !errors.Is(err, edit.ErrJournal) || !strings.Contains(err.Error(), "a.canon") || len(lines) != 0 {
		t.Fatalf("Recover: %v, logged %v", err, lines)
	}
	d.files["/p/a.canon"] = []byte("a new\n")
	if lines, err := warnings(t, d); err != nil || len(lines) != 1 || string(d.files["/p/a.canon"]) != "a old\n" {
		t.Fatalf("Recover: %v, logged %v", err, lines)
	}
}

// indexOf is the place of entry in log, -1 for none.
func indexOf(log []string, entry string) int { return slices.Index(log, entry) }

// log-2026-09-29 M4 U4c-r: where the FS syncs directories, the journal's directory and those
// made for it are synced after the journal and before any rename; every touched directory
// still there is synced after the last change and before the journal goes.
func TestCommitSync(t *testing.T) {
	d, changes := commitFixture()
	s := newSyncFS(d)
	if err := commitIn(s, changes); err != nil {
		t.Fatal(err)
	}
	wrote, firstRename := indexOf(s.log, opWrite+" "+journalFile), indexOf(s.log, opRename+" /p/a.canon")
	for _, dir := range []string{"/p/.canon/journal", "/p/.canon", "/p"} {
		if i := indexOf(s.log, opSync+" "+dir); i < wrote || i > firstRename {
			t.Fatalf("sync %s at %d, journal %d, first rename %d: %v", dir, i, wrote, firstRename, s.log)
		}
	}
	lastChange, gone := indexOf(s.log, opRemove+" /p/pkg/sub"), indexOf(s.log, opRemove+" "+journalFile)
	for _, dir := range []string{"/p", "/p/items", "/p/items/deep", "/p/items/deep/x", "/p/pkg"} {
		i := slices.Index(s.log[lastChange:], opSync+" "+dir)
		if i < 0 || lastChange+i > gone {
			t.Fatalf("sync %s not between %d and %d: %v", dir, lastChange, gone, s.log)
		}
	}
}

// log-2026-09-29 M4 U4c-r: permissions 0o000 are known permissions, journaled and restored.
func TestCommitModeZero(t *testing.T) {
	d := newDiskFS(map[string]string{"/p/a.canon": "a old\n", "/p/b.canon": "b old\n"})
	d.modes["/p/a.canon"] = 0
	before := d.state()
	changes := []edit.Change{
		{Kind: edit.ChangeModified, Path: "a.canon", Before: []byte("a old\n"), After: []byte("a new\n")},
		{Kind: edit.ChangeModified, Path: "b.canon", Before: []byte("b old\n"), After: []byte("b new\n")},
	}
	if err := commitIn(&faultFS{diskFS: d, dieAfterRename: 1}, changes); !errors.Is(err, errDead) {
		t.Fatalf("err = %v", err)
	}
	if !bytes.Contains(d.files[journalFile], []byte(`"mode":0,`)) || d.modes["/p/a.canon"] != 0 {
		t.Fatalf("journal %s, mode %v", d.files[journalFile], d.modes["/p/a.canon"])
	}
	if _, err := warnings(t, d); err != nil {
		t.Fatal(err)
	}
	mustState(t, d.state(), before)
}

// log-2026-09-29 M4 U4c-r: a Stat that fails other than for absence is that failure, not ErrJournal.
func TestCommitStatError(t *testing.T) {
	d, changes := commitFixture()
	before := d.state()
	err := commitIn(statFailFS{diskFS: d, name: "/p/pkg/sub"}, changes)
	if !errors.Is(err, fs.ErrPermission) || errors.Is(err, edit.ErrJournal) {
		t.Fatalf("err = %v", err)
	}
	mustState(t, d.state(), before)
}

// API.md N10, N11, O5: on an FS that sets no permissions, a commit, and a death at any write
// followed by Recover, still leave every file's content as it should be.
func TestCommitNoModes(t *testing.T) {
	contents := func(d *diskFS) map[string]string {
		out := map[string]string{}
		for _, name := range slices.Sorted(maps.Keys(d.files)) {
			out[name] = string(d.files[name])
		}
		return out
	}
	d, changes := commitFixture()
	want := contents(d)
	if err := commitIn(plainFS{d}, changes); err != nil || string(d.files["/p/items/new.canon"]) != "new key\n" {
		t.Fatalf("err = %v", err)
	}
	clean, changes := commitFixture()
	counter := &faultFS{diskFS: clean}
	if err := commitIn(plainFS{counter}, changes); err != nil {
		t.Fatal(err)
	}
	for die := 1; die <= counter.ops; die++ {
		d, changes := commitFixture()
		_ = commitIn(plainFS{&faultFS{diskFS: d, dieAtOp: die}}, changes)
		if err := edit.Recover(siteOf(plainFS{d}), nil); err != nil {
			t.Fatal(err)
		}
		if got := contents(d); !reflect.DeepEqual(got, want) {
			t.Fatalf("die %d: %v", die, got)
		}
	}
}
