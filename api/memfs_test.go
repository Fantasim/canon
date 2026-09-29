package canon_test

import (
	"errors"
	"io/fs"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"testing/fstest"

	canon "github.com/fantasim/canonlang/api"
)

// exampleRoot is where the Examples see the repository's examples/ project.
const exampleRoot = "/examples"

// exampleRoots redirects every root of examples/project.canon (examples/_fixtures/README.md):
// the read roots to their fixtures, the written ones into memory, the inner ones unchanged.
var exampleRoots = map[string]string{
	"resource":  "_fixtures/resource",
	"client":    "_fixtures/client",
	"source":    "/out/source",
	"services":  "/out/services",
	"sovcommon": "/out/sovcommon",
	"web":       "/out/web",
	"parity":    "/out/parity",
	"generated": "/out/generated",
}

// exampleOptions are the options of every Example: the in-memory copy of examples/, every root
// redirected, no cache, so an Example reads nothing outside the repository and writes nothing.
func exampleOptions() canon.Options {
	return canon.Options{FS: newMemFS(committedExamples()), Roots: exampleRoots, Cache: "off"}
}

// committedExamples is the repository's examples/ tree, read once, by absolute name.
var committedExamples = sync.OnceValue(func() map[string][]byte {
	files := map[string][]byte{}
	tree := os.DirFS("../examples")
	_ = fs.WalkDir(tree, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := fs.ReadFile(tree, name)
		files[path.Join(exampleRoot, name)] = data
		return err
	})
	return files
})

// memFS is a canon.FS held in memory; names are absolute and `/`-separated (API.md §2.2).
type memFS struct {
	mu    sync.Mutex
	files map[string][]byte
	dirs  map[string]bool
}

func newMemFS(files map[string][]byte) *memFS {
	m := &memFS{files: maps.Clone(files), dirs: map[string]bool{"/": true}}
	//canon:unordered each file only adds its own parent directories to a set
	for name := range files {
		m.addParents(name)
	}
	return m
}

// dropVolume is name without its volume: memFS holds one, and on Windows Open makes a rooted
// "/law" absolute on the current drive, "D:/law".
func dropVolume(name string) string {
	return name[len(filepath.VolumeName(name)):]
}

func (m *memFS) addParents(name string) {
	for d := path.Dir(name); !m.dirs[d]; d = path.Dir(d) {
		m.dirs[d] = true
	}
}

func (m *memFS) ReadFile(name string) ([]byte, error) {
	name = dropVolume(name)
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.files[name]
	if !ok {
		return nil, &fs.PathError{Op: "read", Path: name, Err: fs.ErrNotExist}
	}
	return slices.Clone(data), nil
}

func (m *memFS) Stat(name string) (fs.FileInfo, error) {
	name = dropVolume(name)
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stat(name)
}

func (m *memFS) stat(name string) (fs.FileInfo, error) {
	if data, ok := m.files[name]; ok {
		return fstest.MapFS{path.Base(name): {Data: data}}.Stat(path.Base(name))
	}
	if m.dirs[name] {
		return fstest.MapFS{path.Base(name): {Mode: fs.ModeDir}}.Stat(path.Base(name))
	}
	return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
}

// ReadDir lists the children of a directory, sorted by name.
func (m *memFS) ReadDir(name string) ([]fs.DirEntry, error) {
	name = dropVolume(name)
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.dirs[name] {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrNotExist}
	}
	var names []string
	for _, all := range []map[string]bool{m.dirs, fileSet(m.files)} {
		//canon:unordered the names are sorted below
		for child := range all {
			if child != name && path.Dir(child) == name {
				names = append(names, child)
			}
		}
	}
	slices.Sort(names)
	out := make([]fs.DirEntry, 0, len(names))
	for _, child := range names {
		info, err := m.stat(child)
		if err != nil {
			return nil, err
		}
		out = append(out, fs.FileInfoToDirEntry(info))
	}
	return out, nil
}

func fileSet(files map[string][]byte) map[string]bool {
	set := map[string]bool{}
	//canon:unordered builds a set
	for name := range files {
		set[name] = true
	}
	return set
}

func (m *memFS) WriteFile(name string, data []byte) error {
	name = dropVolume(name)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.files[name] = slices.Clone(data)
	m.addParents(name)
	return nil
}

func (m *memFS) Rename(oldname, newname string) error {
	oldname, newname = dropVolume(oldname), dropVolume(newname)
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.files[oldname]
	if !ok {
		return &fs.PathError{Op: "rename", Path: oldname, Err: fs.ErrNotExist}
	}
	delete(m.files, oldname)
	m.files[newname] = data
	m.addParents(newname)
	return nil
}

func (m *memFS) Remove(name string) error {
	name = dropVolume(name)
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.files[name]; !ok {
		return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrNotExist}
	}
	delete(m.files, name)
	return nil
}

func (m *memFS) MkdirAll(name string) error {
	name = dropVolume(name)
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.files[name]; ok {
		return &fs.PathError{Op: "mkdir", Path: name, Err: fs.ErrExist}
	}
	m.dirs[name] = true
	m.addParents(name)
	return nil
}

// API.md §2.2: the Examples' FS sorts listings, keeps writes in memory, holds examples/.
func TestExampleFS(t *testing.T) {
	m := newMemFS(committedExamples())
	if _, err := m.Stat(path.Join(exampleRoot, "project.canon")); err != nil {
		t.Fatal(err)
	}
	for _, step := range []error{
		m.MkdirAll("/out/source/x"),
		m.WriteFile("/out/source/x/b.h", []byte("b")),
		m.WriteFile("/out/source/x/a.h", []byte("a")),
		m.Rename("/out/source/x/b.h", "/out/source/c.h"),
		m.Remove("/out/source/x/a.h"),
	} {
		if step != nil {
			t.Fatal(step)
		}
	}
	entries, err := m.ReadDir("/out/source")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range entries {
		got = append(got, e.Name())
	}
	if !slices.Equal(got, []string{"c.h", "x"}) || !entries[1].IsDir() {
		t.Errorf("ReadDir = %v", got)
	}
	if _, err := m.ReadFile("/out/source/x/a.h"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("a removed file reads: %v", err)
	}
}
