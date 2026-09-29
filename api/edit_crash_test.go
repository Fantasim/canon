package canon_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"maps"
	"path"
	"slices"
	"strings"
	"sync"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// errFault is the failure faultFS injects.
var errFault = errors.New("injected fault")

// faultFS fails its rename number failAt, counted from 1; with crash every write after it fails
// too, as for a process that died there; with panics that rename panics instead, once.
type faultFS struct {
	*memFS
	mu      sync.Mutex
	renames int
	failAt  int
	crash   bool
	panics  bool
	dead    bool
}

func (f *faultFS) fault() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.dead {
		return errFault
	}
	return nil
}

func (f *faultFS) Rename(oldname, newname string) error {
	f.mu.Lock()
	f.renames++
	hit := f.renames == f.failAt && !f.dead
	f.dead = f.dead || hit && f.crash
	f.mu.Unlock()
	switch {
	case hit && f.panics:
		panic(errFault)
	case hit:
		return errFault
	}
	if err := f.fault(); err != nil {
		return err
	}
	return f.memFS.Rename(oldname, newname)
}

func (f *faultFS) WriteFile(name string, data []byte) error {
	if err := f.fault(); err != nil {
		return err
	}
	return f.memFS.WriteFile(name, data)
}

func (f *faultFS) Remove(name string) error {
	if err := f.fault(); err != nil {
		return err
	}
	return f.memFS.Remove(name)
}

func (f *faultFS) MkdirAll(name string) error {
	if err := f.fault(); err != nil {
		return err
	}
	return f.memFS.MkdirAll(name)
}

// threeFiles is an edit writing a/a.canon, a/canon.lock (API.md E20) and c/c.canon.
var threeFiles = canon.Edit{Ops: []canon.Op{
	canon.Set("a:config.port", canon.Int(9000)),
	canon.AddEntry("a:codes", canon.Key("second"), canon.Obj{"label": canon.Str("Second")}),
	canon.Set("c:n", canon.Int(2)),
}}

// lawFS is editLaw in memory, with the files of extras added.
func lawFS(extras ...map[string]string) *memFS {
	law := map[string][]byte{}
	for _, files := range append([]map[string]string{editLaw}, extras...) {
		//canon:unordered each file is stored under its own name
		for name, text := range files {
			law["/law/"+name] = []byte(text)
		}
	}
	return newMemFS(law)
}

// dirMark stands for a directory in sources.
const dirMark = "<dir>"

// sources is every file and directory of m outside the project's state directory.
func sources(m *memFS) map[string]string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := map[string]string{}
	//canon:unordered each file is copied under its own name
	for name, data := range m.files {
		if !strings.HasPrefix(name, "/law/.canon") {
			out[name] = string(data)
		}
	}
	//canon:unordered each directory is marked under its own name
	for name := range m.dirs {
		if !strings.HasPrefix(name, "/law/.canon") {
			out[name] = dirMark
		}
	}
	return out
}

// journals are the journal files m holds.
func journals(m *memFS) []string {
	entries, _ := m.ReadDir("/law/.canon/journal")
	var out []string
	for _, e := range entries {
		out = append(out, path.Join("/law/.canon/journal", e.Name()))
	}
	return out
}

// M4 acceptance 5, API.md N10, API.md N11, API.md O5: an FS that fails
// at each rename in turn leaves, after Open, every file as before the edit, whether the edit
// rolled back itself or died there; no journal is left.
func TestEditFailsAtEachRename(t *testing.T) {
	for _, c := range crashCases {
		for _, crash := range []bool{false, true} {
			failEach(t, c, crash)
		}
	}
}

// failEach runs c failing at each rename in turn, until one past the last completes it.
func failEach(t *testing.T, c crashCase, crash bool) {
	t.Helper()
	for k := 1; !failAt(t, c, k, crash); k++ {
		if k > c.renames {
			t.Fatalf("%s, crash %v: rename %d failed", c.name, crash, k)
		}
	}
}

// crashCase is an edit the crash test breaks: its project added to editLaw, the edit, and the
// files it writes, with as many renames, when nothing fails.
type crashCase struct {
	name    string
	law     map[string]string
	edit    canon.Edit
	changes []string
	renames int
}

// filedLaw is a package whose table keeps each entry in a file of a directory still to create.
var filedLaw = map[string]string{
	"e/e.canon": "/// E.\npackage e\n\n/// An entry.\nrecord Entry {\n  /// A value.\n  v: Int = 0\n}\n\n" +
		"/// Entries, each in its own file.\n@files(\"items/{id}.canon\")\nlet filed: table Entry = {}\n",
}

var crashCases = []crashCase{
	{"three files", nil, threeFiles, []string{"a/a.canon", "a/canon.lock", "c/c.canon"}, 3},
	{"a new entry file", filedLaw, canon.Edit{Ops: []canon.Op{canon.AddEntry("e:filed", canon.Key("x"), canon.Obj{"v": canon.Int(1)})}},
		[]string{"e/items/x.canon"}, 1},
}

// failAt runs c on an FS failing at rename k, then opens the project again; true when the edit
// made fewer renames and so completed, as many as c says, writing c's files.
func failAt(t *testing.T, c crashCase, k int, crash bool) bool {
	t.Helper()
	base := lawFS(c.law)
	before := sources(base)
	fsys := &faultFS{memFS: base, failAt: k, crash: crash}
	p, err := canon.Open("/law", canon.Options{FS: fsys})
	if err != nil {
		t.Fatal(err)
	}
	res, err := p.Edit(context.Background(), c.edit)
	_ = p.Close()
	if fsys.renames < k {
		var paths []string
		if res != nil {
			for _, fc := range res.Changes {
				paths = append(paths, fc.Path)
			}
		}
		if err != nil || fsys.renames != c.renames || !slices.Equal(paths, c.changes) {
			t.Fatalf("%s: no rename failed: %d renames, changes %v, %v", c.name, fsys.renames, paths, err)
		}
		return true
	}
	if !errors.Is(err, errFault) {
		t.Errorf("rename %d (crash %v) failed and the edit gave %v", k, crash, err)
	}
	if !crash && !maps.Equal(sources(base), before) {
		t.Errorf("API.md N11: rename %d failed and the rollback left a change", k)
	}
	left := len(journals(base)) // the crash's journal, whole: an unfinished edit (log-2026-09-29 M4 U5b)
	var logs bytes.Buffer
	again, err := canon.Open("/law", canon.Options{FS: base, Logger: slog.New(slog.NewTextHandler(&logs, nil))})
	if err != nil {
		t.Fatalf("API.md O5: Open after rename %d (crash %v): %v", k, crash, err)
	}
	_ = again.Close()
	if warns := strings.Count(logs.String(), "level=WARN"); warns != left || left != 0 && !crash {
		t.Errorf("API.md O5: rename %d (crash %v), %d journals left: the rollback logged %q", k, crash, left, logs.String())
	}
	if got := sources(base); !maps.Equal(got, before) || len(journals(base)) != 0 {
		t.Errorf("API.md O5: rename %d (crash %v): files %v, journals %v", k, crash, diffKeys(got, before), journals(base))
	}
	return false
}

// diffKeys are the names whose content differs between a and b.
func diffKeys(a, b map[string]string) []string {
	var out []string
	for _, name := range slices.Sorted(maps.Keys(a)) {
		if b[name] != a[name] {
			out = append(out, name)
		}
	}
	return out
}

// API.md O5: a journal written on another machine is kept and never applied: Open fails
// with ErrProject naming the journal and why.
func TestOpenForeignJournal(t *testing.T) {
	base := lawFS()
	fsys := &faultFS{memFS: base, failAt: 2, crash: true}
	p, err := canon.Open("/law", canon.Options{FS: fsys})
	if err != nil {
		t.Fatal(err)
	}
	_, _ = p.Edit(context.Background(), threeFiles)
	_ = p.Close()
	names := journals(base)
	if len(names) != 1 {
		t.Fatalf("journals %v", names)
	}
	data, _ := base.ReadFile(names[0])
	var j map[string]any
	if err := json.Unmarshal(data, &j); err != nil {
		t.Fatal(err)
	}
	j["host"] = "elsewhere"
	if data, err = json.Marshal(j); err != nil {
		t.Fatal(err)
	}
	writeLaw(t, base, ".canon/journal/"+path.Base(names[0]), string(data))
	crashed := sources(base)
	_, err = canon.Open("/law", canon.Options{FS: base})
	if !errors.Is(err, canon.ErrProject) || !strings.Contains(err.Error(), path.Base(names[0])) {
		t.Fatalf("Open: %v", err)
	}
	if !maps.Equal(sources(base), crashed) || len(journals(base)) != 1 {
		t.Error("API.md O5: a foreign journal was applied or removed")
	}
}

// API.md X2, API.md N11: a panic while the edit renames is an *InternalError; what it changed is
// rolled back before the call returns, and the project stays usable.
func TestEditPanicInCommit(t *testing.T) {
	base := lawFS()
	before := sources(base)
	fsys := &faultFS{memFS: base, failAt: 2, panics: true}
	p, err := canon.Open("/law", canon.Options{FS: fsys})
	if err != nil {
		t.Fatal(err)
	}
	defer p.Close()
	_, err = p.Edit(context.Background(), threeFiles)
	var ie *canon.InternalError
	if !errors.As(err, &ie) || !errors.Is(err, canon.ErrInternal) {
		t.Fatalf("Edit: %v", err)
	}
	if got := sources(base); !maps.Equal(got, before) || len(journals(base)) != 0 {
		t.Errorf("API.md X2: files %v, journals %v", diffKeys(got, before), journals(base))
	}
	if _, err := p.Edit(context.Background(), threeFiles); err != nil {
		t.Errorf("API.md X2: the project is not usable after the panic: %v", err)
	}
}
