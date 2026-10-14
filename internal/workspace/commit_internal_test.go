package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/writelock"
)

const commitRule = "DECISIONS 375, API.md N11: "

// errPanicked is what a commit cut short by panicFS dies of.
var errPanicked = errors.New("panicked mid-commit")

// panicFS is the OS's files, panicking once, after the first rename outside the journal directory.
type panicFS struct {
	build.WriteFS
	journal string
	done    bool
}

func (f *panicFS) Rename(oldname, newname string) error {
	if err := f.WriteFS.Rename(oldname, newname); err != nil {
		return err
	}
	if !f.done && !strings.HasPrefix(newname, f.journal) {
		f.done = true
		panic(errPanicked)
	}
	return nil
}

// srcOf is package pkg holding n.
func srcOf(pkg string) string {
	return "/// " + pkg + ".\npackage " + pkg + "\n\n/// N.\nconst N = 1\n"
}

// osProject is packages a and b at a temporary directory, with a dead commit's journal recording
// b's lock and no file, which only a recovery taking b's lock would remove.
func osProject(t *testing.T) (string, *build.Project) {
	t.Helper()
	dir := filepath.ToSlash(t.TempDir())
	files := map[string]string{"project.canon": "project acme {\n  canon: \"0.1\"\n}\n", "a/a.canon": srcOf("a"), "b/b.canon": srcOf("b")}
	text := `{"version":2,"revision":"sha256:` + strings.Repeat("e", 64) + `","host":"` + self(dir).Host + `","pid":1,"locks":["b"],"files":[]}`
	files[edit.JournalDir+"/"+strings.Repeat("e", 64)+".json"] = text
	for rel, text := range files {
		name := filepath.Join(filepath.FromSlash(dir), filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(name, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	b, err := build.Open(build.OS(), dir, build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	return dir, b
}

// osText is the file rel of dir, "" when it cannot be read.
func osText(dir, rel string) string {
	data, err := os.ReadFile(filepath.Join(filepath.FromSlash(dir), filepath.FromSlash(rel)))
	if err != nil {
		return ""
	}
	return string(data)
}

// setN sets a's N to 2.
func setN(dir string) []edit.Change {
	before := osText(dir, "a/a.canon")
	return []edit.Change{{Kind: edit.ChangeModified, Path: "a/a.canon", Before: []byte(before), After: []byte(strings.Replace(before, "N = 1", "N = 2", 1))}}
}

// panicked is what running fn panicked with, nil for none.
func panicked(fn func() error) (r any) {
	defer func() { r = recover() }()
	_ = fn()
	return nil
}

// commitRev is the revision the tests' commits name their journal by.
var commitRev = "sha256:" + strings.Repeat("f", 64)

// API.md X2, N11 (DECISIONS 375: J8): a commit recording locks that panics after its first rename
// rolls back its own journal under the locks it still holds, and leaves another package's.
func TestCommitPanicSettlesOwn(t *testing.T) {
	if !writelock.Lockable() {
		t.Skip("no writer locks on this platform")
	}
	dir, b := osProject(t)
	before := osText(dir, "a/a.canon")
	held, err := writelock.Acquire(context.Background(), filepath.FromSlash(dir), writelock.ExclusiveOf("a"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = held.Release() }()
	jdir := project.Join(dir, edit.JournalDir)
	site := edit.Site{FS: &panicFS{WriteFS: build.OS(), journal: jdir}, Layout: b, Self: self(dir), Locks: lockerFor(build.OS(), dir), Held: []string{"a"}}
	if r := panicked(func() error { return commitOwn(context.Background(), site, commitRev, setN(dir)) }); r != errPanicked {
		t.Fatalf("%sthe commit ended with %v", commitRule, r)
	}
	if osText(dir, "a/a.canon") != before || osText(dir, edit.JournalDir+"/"+strings.Repeat("f", 64)+".json") != "" {
		t.Errorf("%sthe panicked commit was not rolled back under its locks", commitRule)
	}
	if osText(dir, edit.JournalDir+"/"+strings.Repeat("e", 64)+".json") == "" {
		t.Errorf("%sthe panic path rolled back another package's journal", commitRule)
	}
}

// DECISIONS 375 (J7): a commit writing a file no package owns takes J7's locks, waiting for them,
// records none, and is refused by any journal, one of a package it does not lock included.
func TestCommitUnownedLockless(t *testing.T) {
	if !writelock.Lockable() {
		t.Skip("no writer locks on this platform")
	}
	dir, b := osProject(t)
	p := New(b)
	defer p.Close()
	s, err := p.Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	held, err := writelock.Acquire(context.Background(), filepath.FromSlash(dir), writelock.ExclusiveOf("a"))
	if err != nil {
		t.Fatal(err)
	}
	o := &EditOutcome{Plan: &edit.Plan{Touched: []string{"a"}, Unowned: []string{"loose.json"}}, Changes: setN(dir), Before: s, After: s}
	waits, errs := make(chan writelock.Wait, 1), make(chan error, 1)
	ctx := writelock.WithWaiting(context.Background(), func(w writelock.Wait) { waits <- w })
	go func() { errs <- s.commit(ctx, o, []string{"a"}) }()
	select {
	case w := <-waits:
		if w.Package != "a" {
			t.Errorf("%sthe commit waited for %+v, want a", commitRule, w)
		}
	case <-time.After(time.Minute):
		t.Fatal("the unowned commit took no lock")
	}
	if err := held.Release(); err != nil {
		t.Fatal(err)
	}
	if err := <-errs; !errors.Is(err, edit.ErrJournal) {
		t.Errorf("%sthe unowned commit beside b's journal: %v, want ErrJournal", commitRule, err)
	}
	unownedJournal(t, dir, s)
}

// unownedJournal proves an unowned commit's site holds a's lock but records none in its journal.
func unownedJournal(t *testing.T, dir string, s *Snapshot) {
	t.Helper()
	site := edit.Site{Layout: s.b, Self: self(dir)}
	release, reqs, err := s.lockWriters(context.Background(), &site, []string{"a"}, true)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = release() }()
	if _, ok, _ := writelock.TryAcquire(filepath.FromSlash(dir), writelock.ExclusiveOf("a")); ok || site.Locks != nil || reqs != nil {
		t.Fatalf("%sunowned: a free %v, Locker %v, requests %v", commitRule, ok, site.Locks, reqs)
	}
	if err := os.Remove(filepath.Join(filepath.FromSlash(dir), filepath.FromSlash(edit.JournalDir), strings.Repeat("e", 64)+".json")); err != nil {
		t.Fatal(err)
	}
	site.FS = &panicFS{WriteFS: build.OS(), journal: project.Join(dir, edit.JournalDir)}
	if r := panicked(func() error { return edit.Commit(context.Background(), site, commitRev, setN(dir)) }); r != errPanicked {
		t.Fatalf("%sthe commit ended with %v", commitRule, r)
	}
	var j map[string]any
	if err := json.Unmarshal([]byte(osText(dir, edit.JournalDir+"/"+strings.Repeat("f", 64)+".json")), &j); err != nil {
		t.Fatal(err)
	}
	if _, ok := j["locks"]; ok {
		t.Errorf("%san unowned commit's journal records locks: %v", commitRule, j["locks"])
	}
}
