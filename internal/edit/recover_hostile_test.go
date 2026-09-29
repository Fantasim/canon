package edit_test

import (
	"encoding/json"
	"errors"
	"path"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/edit"
)

// hostile is a way to tamper with a journal and the disk around it.
type hostile struct {
	name   string
	mutate func(j map[string]any)
	setup  func(d *diskFS)
	want   string // the refusal's reason
}

// The reasons a hostile journal is refused for.
const (
	whyOutside = "outside the project"
	whyHidden  = "hidden path"
	whyKind    = "not a file the edit API writes"
	whyMode    = "mode beyond"
	whyLink    = "symbolic link"
)

func file0(key string, v any) func(map[string]any) {
	return func(j map[string]any) { j["files"].([]any)[0].(map[string]any)[key] = v }
}

func setKey(key string, v any) func(map[string]any) {
	return func(j map[string]any) { j[key] = v }
}

// addFile adds a file entry to the journal, and, when created is set, a created directory.
func addFile(entry map[string]any, created string) func(map[string]any) {
	return func(j map[string]any) {
		j["files"] = append(j["files"].([]any), entry)
		if created != "" {
			j["created"] = []any{created}
		}
	}
}

// plant puts text at name, so a journal's digest gate passes and only the fault under test refuses it.
func plant(name, text string) func(d *diskFS) {
	return func(d *diskFS) {
		d.mkdirs(path.Dir(name))
		d.files[name] = []byte(text)
	}
}

// outside is a directory outside the project, reached from it through the link /p/lnk.
func outside(d *diskFS) {
	d.mkdirs("/outside")
	d.files["/outside/keep.canon"] = []byte("keep\n")
	d.link("/p/lnk", "/outside")
}

func both(a, b func(d *diskFS)) func(d *diskFS) {
	return func(d *diskFS) { a(d); b(d) }
}

// hostiles are journals a hostile repository could carry (log-2026-09-29 M4 U4c-r, U4c-r3).
var hostiles = []hostile{
	{"parent", file0("path", "../outside.canon"), plant("/outside.canon", "a new\n"), whyOutside},
	{"inner parent", file0("path", "items/../../outside.canon"), plant("/outside.canon", "a new\n"), whyOutside},
	{"absolute outside", file0("path", "/etc/passwd"), plant("/etc/passwd", "a new\n"), whyOutside},
	{"a root itself", file0("path", "/data"), plant("/data", "a new\n"), whyOutside},
	{"the project", file0("path", "."), nil, whyOutside},
	{"canon dir", file0("path", ".canon/journal/x.json"), plant("/p/.canon/journal/x.json", "a new\n"), whyHidden},
	{"git hook", file0("path", ".git/hooks/post-checkout"), plant("/p/.git/hooks/post-checkout", "a new\n"), whyHidden},
	{"hidden file", file0("path", "items/.x.canon"), plant("/p/items/.x.canon", "a new\n"), whyHidden},
	{"script", file0("path", "x.sh"), plant("/p/x.sh", "a new\n"), whyKind},
	{"setuid", file0("mode", 0o4755), nil, whyMode},
	{"exec", file0("mode", 0o755), nil, whyMode},
	{"digest", file0("new", "zz"), nil, "invalid digest"},
	{"foreign host", setKey("host", "host-b"), nil, "another machine"},
	{"created outside", setKey("created", []any{"/elsewhere"}), nil, whyOutside},
	{"created above no new file", setKey("created", []any{"pkg"}), nil, "above no new file"},
	{"created project", setKey("created", []any{"."}), nil, whyOutside},
	{"created holding others", addFile(map[string]any{"path": "pkg/evil.canon", "existed": false, "new": sum("x")}, "pkg"), nil, "not the edit's: /p/pkg/pkg.canon"},
	{"removed outside", setKey("removed", []any{map[string]any{"path": "../x"}}), nil, whyOutside},
	{"removed setuid", setKey("removed", []any{map[string]any{"path": "pkg/sub", "mode": 0o4755}}), nil, whyMode},
	{"version", setKey("version", 9), nil, "unknown version"},
	{"link created", addFile(map[string]any{"path": "lnk/new.canon", "existed": false, "new": sum("n\n")}, "lnk"),
		both(outside, plant("/outside/new.canon", "n\n")), whyLink},
	{"link planted", addFile(map[string]any{"path": "lnk/planted.canon", "existed": true, "old": []byte("evil\n"), "new": sum("theirs\n")}, ""),
		both(outside, plant("/outside/planted.canon", "theirs\n")), whyLink},
	{"link planted txt", addFile(map[string]any{"path": "lnk/planted.txt", "existed": true, "old": []byte("evil\n"), "new": sum("theirs\n")}, ""),
		both(outside, plant("/outside/planted.txt", "theirs\n")), whyKind},
	{"dangling link", addFile(map[string]any{"path": "lnk/x.canon", "existed": true, "old": []byte("evil\n"), "new": "absent"}, ""),
		func(d *diskFS) { d.link("/p/lnk", "/nowhere") }, whyLink},
}

// log-2026-09-29 M4 U4c-r, U4c-r3: every hostile journal is refused whole, naming why: it stays,
// ErrJournal, nothing is written or logged, the files outside the project (in the state) untouched.
func TestRecoverRefusesJournal(t *testing.T) {
	for _, tc := range hostiles {
		t.Run(tc.name, func(t *testing.T) {
			d, _ := crashed(t)
			if tc.setup != nil {
				tc.setup(d)
			}
			var j map[string]any
			if err := json.Unmarshal(d.files[journalFile], &j); err != nil {
				t.Fatal(err)
			}
			tc.mutate(j)
			data, _ := json.Marshal(j)
			d.files[journalFile] = data
			before := d.state()
			lines, err := warnings(t, d)
			if !errors.Is(err, edit.ErrJournal) || !strings.Contains(err.Error(), tc.want) || len(lines) != 0 {
				t.Fatalf("logged %v, err %v, want %q", lines, err, tc.want)
			}
			mustState(t, d.state(), before)
		})
	}
}

// log-2026-09-29 M4 U4c-r3: on an FS that resolves no link, a journal of another machine is
// still refused, and this machine's is applied (without permissions: that FS sets none).
func TestRecoverNoLinkCapability(t *testing.T) {
	d, _ := crashed(t)
	crash := d.state()
	foreign := siteOf(plainFS{d})
	foreign.Self.Host = "host-b"
	if err := edit.Recover(foreign, nil); !errors.Is(err, edit.ErrJournal) {
		t.Fatalf("err = %v", err)
	}
	mustState(t, d.state(), crash)
	if err := edit.Recover(siteOf(plainFS{d}), nil); err != nil {
		t.Fatal(err)
	}
	if string(d.files["/p/a.canon"]) != "a old\n" || len(journalsIn(d)) != 0 {
		t.Fatalf("a.canon %q, journals %v", d.files["/p/a.canon"], journalsIn(d))
	}
}

// log-2026-09-29 M4 U4c-r3: clearing a directory never descends a symbolic link in it: the link
// goes, what it leads to stays.
func TestRemoveAllKeepsLinkTargets(t *testing.T) {
	d := newDiskFS(map[string]string{"/p/new/a.canon": "a\n", "/outside/keep.canon": "keep\n"})
	d.link("/p/new/lnk", "/outside")
	if err := edit.RemoveAll(d, "/p/new"); err != nil {
		t.Fatal(err)
	}
	if d.dirs["/p/new"] || len(d.links) != 0 || string(d.files["/outside/keep.canon"]) != "keep\n" {
		t.Fatalf("state:\n%s", d.state())
	}
}

// log-2026-09-29 M4 U4c-r3, U5b-r: a commit whose files lie through a symbolic link below the
// project is refused as ErrUnwritable before anything is written: its rollback could not follow them.
func TestCommitThroughLink(t *testing.T) {
	d := newDiskFS(map[string]string{"/p/real/a.canon": "a old\n"})
	d.link("/p/items", "/p/real")
	before := d.state()
	changes := []edit.Change{{Kind: edit.ChangeModified, Path: "items/a.canon", Before: []byte("a old\n"), After: []byte("a new\n")}}
	if err := commitIn(d, changes); !errors.Is(err, edit.ErrUnwritable) || errors.Is(err, edit.ErrJournal) {
		t.Fatalf("err = %v", err)
	}
	mustState(t, d.state(), before)
}

// log-2026-09-29 M4 U4c-r3: a project directory that is itself reached through a link commits
// and recovers: only links below the project directory or a root are refused.
func TestCommitProjectBehindLink(t *testing.T) {
	d := newDiskFS(map[string]string{"/real/p/a.canon": "a old\n", "/real/p/b.canon": "b old\n"})
	d.link("/p", "/real/p")
	before := d.state()
	changes := []edit.Change{
		{Kind: edit.ChangeModified, Path: "a.canon", Before: []byte("a old\n"), After: []byte("a new\n")},
		{Kind: edit.ChangeModified, Path: "b.canon", Before: []byte("b old\n"), After: []byte("b new\n")},
	}
	if err := commitIn(&faultFS{diskFS: d, dieAfterRename: 1}, changes); !errors.Is(err, errDead) {
		t.Fatalf("err = %v", err)
	}
	if err := edit.Recover(siteOf(d), nil); err != nil {
		t.Fatal(err)
	}
	mustState(t, d.state(), before)
}
