package workspace

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/writelock"
)

const settlerRule = "DECISIONS 375, ADR-0021 J4: "

// dirLayout places a project at dir.
type dirLayout string

func (d dirLayout) Dir() string                    { return string(d) }
func (d dirLayout) Abs(file string) (string, bool) { return project.Join(string(d), file), true }
func (d dirLayout) Display(abs string) string      { return strings.TrimPrefix(abs, string(d)+pathSep) }

// hidingFS is the OS's files whose journal directory lists empty the first hide times.
type hidingFS struct {
	build.WriteFS
	journal string
	hide    int
}

func (h *hidingFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if name == h.journal && h.hide > 0 {
		h.hide--
		return nil, nil
	}
	return h.WriteFS.ReadDir(name)
}

// journalSite is a project at a temporary directory holding one journal of this host recording
// locks, the site over it on fsys wrapped by wrap.
func journalSite(t *testing.T, locks string, wrap func(build.WriteFS, string) build.WriteFS) edit.Site {
	t.Helper()
	dir := filepath.ToSlash(t.TempDir())
	jdir := project.Join(dir, edit.JournalDir)
	if err := os.MkdirAll(filepath.FromSlash(jdir), 0o755); err != nil {
		t.Fatal(err)
	}
	me := self(dir)
	text := `{"version":2,"revision":"sha256:` + strings.Repeat("d", 64) + `","host":"` + me.Host + `","pid":1,"locks":` + locks + `,"files":[]}`
	if err := os.WriteFile(filepath.FromSlash(project.Join(jdir, strings.Repeat("d", 64)+".json")), []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
	return edit.Site{FS: wrap(build.OS(), jdir), Layout: dirLayout(dir), Self: me, Locks: lockerFor(build.OS(), dir)}
}

func plainOS(w build.WriteFS, _ string) build.WriteFS { return w }

// ADR-0021 J4: Meeting grows held by the journals it meets, else returns held itself.
func TestSettlerMeeting(t *testing.T) {
	if !writelock.Lockable() {
		t.Skip("no writer locks on this platform")
	}
	s := settler{site: journalSite(t, `["a","b"]`, plainOS)}
	shared := []writelock.Request{{Package: "a", Mode: writelock.Shared}}
	got, err := s.Meeting(shared)
	if err != nil || !slices.Equal(exclusive(got), []string{"a", "b"}) {
		t.Errorf("%sMeeting(a shared) = %v, %v", settlerRule, got, err)
	}
	held := writelock.ExclusiveOf("a", "b", "c")
	if got, err := s.Meeting(held); err != nil || !slices.Equal(got, held) {
		t.Errorf("%sMeeting(a b c) = %v, %v, want held unchanged", settlerRule, got, err)
	}
	disjoint := writelock.ExclusiveOf("c")
	if got, err := s.Meeting(disjoint); err != nil || !slices.Equal(got, disjoint) {
		t.Errorf("%sMeeting(c) = %v, %v, want held unchanged", settlerRule, got, err)
	}
}

// ADR-0021 J4, J6: a meeting journal left after Settle fails it with edit.ErrJournal (fail closed).
func TestSettlerFailsClosed(t *testing.T) {
	if !writelock.Lockable() {
		t.Skip("no writer locks on this platform")
	}
	hide := func(w build.WriteFS, jdir string) build.WriteFS { return &hidingFS{WriteFS: w, journal: jdir, hide: 1} }
	s := settler{site: journalSite(t, `["a"]`, hide)}
	_, err := s.Settle(writelock.ExclusiveOf("a"))
	if !errors.Is(err, edit.ErrJournal) || !errors.Is(err, errMeeting) || !strings.Contains(err.Error(), strings.Repeat("d", 64)+".json") {
		t.Errorf("%sSettle beside a journal it left: %v", settlerRule, err)
	}
}

// DECISIONS 375 (J7): an edit locks the package of each canon.lock it writes; a canon.lock and a
// source are no data file, whose readers it would lock.
func TestEditWritersLocks(t *testing.T) {
	o := &EditOutcome{
		Plan:       &edit.Plan{Touched: []string{"b", "a", ""}},
		Changes:    []edit.Change{{Path: "a/a.canon"}, {Path: "z/canon.lock"}},
		wroteLocks: []build.Lock{{Package: "z", Path: "z/canon.lock"}},
	}
	if got := o.writers(nil); !slices.Equal(got, []string{"a", "b", "z"}) {
		t.Errorf("DECISIONS 375, J7: writers %v, want [a b z]", got)
	}
}

// DECISIONS 375 (J12): only the OS's files, on a platform with writer locks, have a Locker.
func TestLockerFor(t *testing.T) {
	if lockerFor(build.OS(), "/p") == nil && writelock.Lockable() {
		t.Error("DECISIONS 375, J12: the OS's files have no Locker")
	}
	if lockerFor(project.OS(), "/p") != nil {
		t.Error("DECISIONS 375, J12: a file system without writer locks has a Locker")
	}
}
