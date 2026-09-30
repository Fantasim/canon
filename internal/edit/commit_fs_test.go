package edit_test

import (
	"errors"
	"io/fs"
	"maps"
	"path"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/project"
)

var (
	errNotEmpty = errors.New("directory not empty")
	errInjected = errors.New("injected failure")
	errDead     = errors.New("process is dead")
)

// faultFS is a diskFS a test breaks. Once dead, the process applies no operation any more;
// a WriteFile it dies in leaves half its bytes, as a write that is not atomic would.
type faultFS struct {
	*diskFS
	mu             sync.Mutex
	ops, renames   int                                // writes and renames so far
	failOp         int                                // the write that fails, once, not applied
	failRename     int                                // the rename that fails, once
	dieAfterRename int                                // the rename right after which the process dies
	dieAtOp        int                                // the write the process dies in
	dead           bool                               // no operation is applied any more
	onOp           func(op, name string, data []byte) // sees every write before it is applied
}

// write counts one write and says whether it may go ahead; torn is a WriteFile it dies in.
func (f *faultFS) write(op, name string, data []byte) (torn bool, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.dead {
		return false, errDead
	}
	f.ops++
	if f.onOp != nil {
		f.onOp(op, name, data)
	}
	switch f.ops {
	case f.dieAtOp:
		f.dead = true
		return op == opWrite, errDead
	case f.failOp:
		return false, errInjected
	}
	return false, nil
}

const (
	opWrite  = "write"
	opRename = "rename"
	opRemove = "remove"
	opMkdir  = "mkdir"
	opChmod  = "chmod"
	opSync   = "sync"
)

func (f *faultFS) isDead() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.dead
}

func (f *faultFS) ReadFile(name string) ([]byte, error) {
	if f.isDead() {
		return nil, errDead
	}
	return f.diskFS.ReadFile(name)
}

func (f *faultFS) Stat(name string) (fs.FileInfo, error) {
	if f.isDead() {
		return nil, errDead
	}
	return f.diskFS.Stat(name)
}

func (f *faultFS) ReadDir(name string) ([]fs.DirEntry, error) {
	if f.isDead() {
		return nil, errDead
	}
	return f.diskFS.ReadDir(name)
}

func (f *faultFS) EvalSymlinks(name string) (string, error) {
	if f.isDead() {
		return "", errDead
	}
	return f.diskFS.EvalSymlinks(name)
}

func (f *faultFS) WriteFile(name string, data []byte) error {
	torn, err := f.write(opWrite, name, data)
	if torn {
		_ = f.diskFS.WriteFile(name, data[:len(data)/2])
	}
	if err != nil {
		return err
	}
	return f.diskFS.WriteFile(name, data)
}

func (f *faultFS) Rename(oldname, newname string) error {
	if _, err := f.write(opRename, newname, nil); err != nil {
		return err
	}
	f.mu.Lock()
	f.renames++
	n := f.renames
	f.mu.Unlock()
	if n == f.failRename {
		return errInjected
	}
	err := f.diskFS.Rename(oldname, newname)
	if n == f.dieAfterRename {
		f.mu.Lock()
		f.dead = true
		f.mu.Unlock()
		return errDead
	}
	return err
}

func (f *faultFS) Remove(name string) error {
	if _, err := f.write(opRemove, name, nil); err != nil {
		return err
	}
	return f.diskFS.Remove(name)
}

func (f *faultFS) MkdirAll(name string) error {
	if _, err := f.write(opMkdir, name, nil); err != nil {
		return err
	}
	return f.diskFS.MkdirAll(name)
}

func (f *faultFS) Chmod(name string, mode fs.FileMode) error {
	if _, err := f.write(opChmod, name, nil); err != nil {
		return err
	}
	return f.diskFS.Chmod(name, mode)
}

// syncFS is a faultFS that syncs directories, as U8's OS FS will: it logs every write and
// sync; a directory that is gone cannot be synced, and failDir fails failTimes times (-1: always).
type syncFS struct {
	*faultFS
	log       []string
	failDir   string
	failTimes int
}

func newSyncFS(d *diskFS) *syncFS {
	s := &syncFS{}
	s.faultFS = &faultFS{diskFS: d, onOp: func(op, name string, _ []byte) { s.log = append(s.log, op+" "+name) }}
	return s
}

func (s *syncFS) SyncDir(dir string) error {
	if s.isDead() {
		return errDead
	}
	if _, err := s.diskFS.Stat(dir); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if dir == s.failDir && s.failTimes != 0 {
		s.failTimes--
		return errInjected
	}
	s.log = append(s.log, opSync+" "+dir)
	return nil
}

// plainFS is a file system that sets no permissions and resolves no link: no Chmod, no EvalSymlinks.
type plainFS struct{ d build.WriteFS }

func (p plainFS) ReadFile(name string) ([]byte, error)       { return p.d.ReadFile(name) }
func (p plainFS) Stat(name string) (fs.FileInfo, error)      { return p.d.Stat(name) }
func (p plainFS) ReadDir(name string) ([]fs.DirEntry, error) { return p.d.ReadDir(name) }
func (p plainFS) WriteFile(name string, data []byte) error   { return p.d.WriteFile(name, data) }
func (p plainFS) Rename(oldname, newname string) error       { return p.d.Rename(oldname, newname) }
func (p plainFS) Remove(name string) error                   { return p.d.Remove(name) }
func (p plainFS) MkdirAll(name string) error                 { return p.d.MkdirAll(name) }

// statFailFS is a diskFS whose Stat of one name fails with a permission error.
type statFailFS struct {
	*diskFS
	name string
}

func (s statFailFS) Stat(name string) (fs.FileInfo, error) {
	if name == s.name {
		return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrPermission}
	}
	return s.diskFS.Stat(name)
}

// layout is a project at dir with declared roots elsewhere, each a name to its directory.
type layout struct {
	dir   string
	roots map[string]string
}

var testLayout = layout{dir: projectDir, roots: map[string]string{"data": "/data"}}

func (l layout) Dir() string { return l.dir }

func (l layout) Abs(file string) (string, bool) {
	name, ok := strings.CutPrefix(file, "@")
	if !ok {
		return project.Join(l.dir, file), file != ""
	}
	root, rest, _ := strings.Cut(name, "/")
	dir, ok := l.roots[root]
	return project.Join(dir, rest), ok
}

func (l layout) Display(abs string) string {
	if rel, ok := strings.CutPrefix(abs, l.dir+"/"); ok {
		return rel
	}
	for _, name := range slices.Sorted(maps.Keys(l.roots)) {
		if abs == l.roots[name] {
			return "@" + name
		}
		if rel, ok := strings.CutPrefix(abs, l.roots[name]+"/"); ok {
			return "@" + name + "/" + rel
		}
	}
	return path.Base(abs)
}

const (
	testHost = "host-a"
	testPID  = 7
)

// siteOf is fsys under the test layout, committed by process 7 of host-a, which is dead.
func siteOf(fsys build.WriteFS) edit.Site {
	return edit.Site{FS: fsys, Layout: testLayout, Self: edit.Process{Host: testHost, PID: testPID}}
}

func mustState(t *testing.T, got, want string) {
	t.Helper()
	if got != want {
		t.Fatalf("file system state:\n%s\nwant:\n%s", got, want)
	}
}
