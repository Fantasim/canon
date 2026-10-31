package workspace

import (
	"context"
	"io/fs"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/fantasim/canonlang/internal/build"
)

// The project of keptFS: x/x.canon, then the same size with other bytes, by display and name.
const (
	keptFile    = "/law/x/x.canon"
	keptDir     = "/law/x"
	keptDisplay = "x/x.canon"
)

// keptChanged is memoPackage with other bytes of the same size.
var keptChanged = strings.Replace(memoPackage, "= 1", "= 3", 1)

// keptFS is writeMapFS behind a lock, so a watch reads it while a test writes. Its stamp is
// Windows': size and modification time, kept by a write, no change time nor identity (DECISIONS
// 374). onStat, when set, runs before each stat.
type keptFS struct {
	mu     sync.Mutex
	w      writeMapFS
	onStat func(name string)
}

func newKeptFS() *keptFS {
	old := time.Unix(1, 0) // long before any read: never racily clean (API.md S1)
	return &keptFS{w: writeMapFS{clockFS: clockFS{MapFS: fstest.MapFS{
		"law/project.canon": {Data: []byte(memoProject), ModTime: old},
		"law/x":             {Mode: fs.ModeDir, ModTime: old},
		"law/x/x.canon":     {Data: []byte(memoPackage), ModTime: old},
	}}}}
}

func (k *keptFS) ReadFile(name string) ([]byte, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.w.ReadFile(name)
}

func (k *keptFS) Stat(name string) (fs.FileInfo, error) {
	k.mu.Lock()
	hook := k.onStat
	k.mu.Unlock()
	if hook != nil {
		hook(name)
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.w.Stat(name)
}

func (k *keptFS) ReadDir(name string) ([]fs.DirEntry, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.w.ReadDir(name)
}

func (k *keptFS) WriteFile(name string, data []byte) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.w.WriteFile(name, data)
}

func (k *keptFS) Rename(oldname, newname string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.w.Rename(oldname, newname)
}

func (k *keptFS) Remove(name string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.w.Remove(name)
}

func (k *keptFS) MkdirAll(name string) error { return k.w.MkdirAll(name) }

func (k *keptFS) setOnStat(hook func(string)) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.onStat = hook
}

// replace writes keptChanged over x/x.canon, keeping its stamp.
func (k *keptFS) replace(t *testing.T) {
	t.Helper()
	if err := k.WriteFile(keptFile, []byte(keptChanged)); err != nil {
		t.Fatal(err)
	}
}

// keptProject is a project over fsys whose snapshot has read x/x.canon and listed x/.
func keptProject(t *testing.T, fsys *keptFS) *Project {
	t.Helper()
	b, err := build.Open(fsys, "/law", build.Options{})
	if err != nil {
		t.Fatal(err)
	}
	p := New(b)
	t.Cleanup(p.Close)
	s, err := p.Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.fs.ReadFile(keptFile); err != nil {
		t.Fatal(err)
	}
	if _, err := s.fs.ReadDir(keptDir); err != nil {
		t.Fatal(err)
	}
	return p
}

// readKept is x/x.canon as the snapshot a Read returns holds it, and that snapshot.
func readKept(t *testing.T, p *Project) (string, *Snapshot) {
	t.Helper()
	s, err := p.Read(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	data, err := s.fs.ReadFile(keptFile)
	if err != nil {
		t.Fatal(err)
	}
	return string(data), s
}
