package workspace_test

import (
	"io/fs"
	"maps"
	"math/rand/v2"
	"path"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/fantasim/canonlang/internal/project"
)

// memFS is a writable FS in memory: later mtimes, shuffled listings (API.md §2.2).
type memFS struct {
	mu     sync.Mutex
	files  map[string][]byte
	mtimes map[string]int64 // files and directories
	dirs   map[string]bool
	clock  int64
	zero   bool // every modification time is the zero time
	reads  map[string]int
	links  map[string]string // symbolic links: a directory name to the one it leads to
}

func newMemFS(files map[string]string) *memFS {
	m := &memFS{files: map[string][]byte{}, mtimes: map[string]int64{}, dirs: map[string]bool{"/": true}, reads: map[string]int{}}
	for _, name := range slices.Sorted(maps.Keys(files)) {
		m.put(name, []byte(files[name]))
	}
	return m
}

// put stores data at name, its directories created, name and its directory stamped later.
func (m *memFS) put(name string, data []byte) {
	m.clock++
	m.files[name] = data
	m.mtimes[name] = m.clock
	for d := project.DirOf(name); ; d = project.DirOf(d) {
		m.dirs[d] = true
		m.mtimes[d] = m.clock
		if d == project.DirOf(d) {
			return
		}
	}
}

func (m *memFS) info(name string) (fs.FileInfo, bool) {
	mt := time.Unix(m.mtimes[name], 0)
	if m.zero {
		mt = time.Time{}
	}
	if data, ok := m.files[name]; ok {
		return fileInfo{name: path.Base(name), size: int64(len(data)), mtime: mt}, true
	}
	if m.dirs[name] {
		return fileInfo{name: path.Base(name), mtime: mt, dir: true}, true
	}
	return nil, false
}

func (m *memFS) ReadFile(name string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.reads[name]++
	data, ok := m.files[name]
	switch {
	case m.dirs[name]: // as the OS does: a directory is not missing, it cannot be read
		return nil, &fs.PathError{Op: "read", Path: name, Err: fs.ErrInvalid}
	case !ok:
		return nil, &fs.PathError{Op: "read", Path: name, Err: fs.ErrNotExist}
	}
	return slices.Clone(data), nil
}

func (m *memFS) Stat(name string) (fs.FileInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if info, ok := m.info(name); ok {
		return info, nil
	}
	return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
}

func (m *memFS) ReadDir(name string) ([]fs.DirEntry, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.dirs[name] {
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrNotExist}
	}
	var out []fs.DirEntry
	for _, all := range []map[string]bool{m.dirs, keySet(m.files)} {
		for child := range all {
			if child != name && project.DirOf(child) == name {
				info, _ := m.info(child)
				out = append(out, fs.FileInfoToDirEntry(info))
			}
		}
	}
	rand.Shuffle(len(out), func(i, j int) { out[i], out[j] = out[j], out[i] })
	return out, nil
}

func keySet(files map[string][]byte) map[string]bool {
	set := map[string]bool{}
	for name := range files {
		set[name] = true
	}
	return set
}

func (m *memFS) WriteFile(name string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.put(name, slices.Clone(data))
	return nil
}

func (m *memFS) Rename(oldname, newname string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.files[oldname]
	if !ok {
		return &fs.PathError{Op: "rename", Path: oldname, Err: fs.ErrNotExist}
	}
	delete(m.files, oldname)
	m.put(newname, data)
	return nil
}

func (m *memFS) Remove(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.files[name]; !ok {
		return &fs.PathError{Op: "remove", Path: name, Err: fs.ErrNotExist}
	}
	delete(m.files, name)
	m.clock++
	m.mtimes[project.DirOf(name)] = m.clock
	return nil
}

func (m *memFS) MkdirAll(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for d := name; !m.dirs[d]; d = project.DirOf(d) {
		m.clock++
		m.dirs[d] = true
		m.mtimes[d] = m.clock
		if d == project.DirOf(d) {
			return nil
		}
	}
	return nil
}

// readsOf is how many times name was read.
func (m *memFS) EvalSymlinks(name string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for link, target := range m.links {
		if name == link || strings.HasPrefix(name, link+"/") {
			return target + name[len(link):], nil
		}
	}
	return name, nil
}

// link points link at target, as a later retarget would.
func (m *memFS) link(link, target string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.links == nil {
		m.links = map[string]string{}
	}
	m.links[link] = target
}

func (m *memFS) readsOf(name string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.reads[name]
}

type fileInfo struct {
	name  string
	size  int64
	mtime time.Time
	dir   bool
}

func (f fileInfo) Name() string       { return f.name }
func (f fileInfo) Size() int64        { return f.size }
func (f fileInfo) ModTime() time.Time { return f.mtime }
func (f fileInfo) IsDir() bool        { return f.dir }
func (f fileInfo) Sys() any           { return nil }
func (f fileInfo) Mode() fs.FileMode {
	if f.dir {
		return fs.ModeDir
	}
	return 0
}
