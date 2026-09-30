package canon

import (
	"io/fs"
	"maps"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing/fstest"
	"time"
)

// writeFS is a writable FS in memory whose Rename, a write's last step, calls renamed.
type writeFS struct {
	mu      sync.Mutex
	files   fstest.MapFS
	clock   int64
	renamed func()
}

func newWriteFS(files map[string][]byte, renamed func()) *writeFS {
	w := &writeFS{files: fstest.MapFS{}, renamed: renamed}
	for _, name := range slices.Sorted(maps.Keys(files)) {
		w.put(name, files[name])
	}
	return w
}

// mapName is name without its volume (Open's "D:/law" on Windows, API.md §2.2) nor leading '/'.
func mapName(name string) string {
	return strings.TrimPrefix(name[len(filepath.VolumeName(name)):], "/")
}

func (w *writeFS) put(name string, data []byte) {
	w.clock++
	w.files[mapName(name)] = &fstest.MapFile{Data: slices.Clone(data), ModTime: time.Unix(w.clock, 0)}
}

func (w *writeFS) ReadFile(name string) ([]byte, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.files.ReadFile(mapName(name))
}

func (w *writeFS) Stat(name string) (fs.FileInfo, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.files.Stat(mapName(name))
}

func (w *writeFS) ReadDir(name string) ([]fs.DirEntry, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.files.ReadDir(mapName(name))
}

func (w *writeFS) WriteFile(name string, data []byte) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.put(name, data)
	return nil
}

func (w *writeFS) Rename(oldname, newname string) error {
	w.mu.Lock()
	old := mapName(oldname)
	f, ok := w.files[old]
	if ok {
		delete(w.files, old)
		w.put(newname, f.Data)
	}
	w.mu.Unlock()
	if !ok {
		return &fs.PathError{Op: "rename", Path: oldname, Err: fs.ErrNotExist}
	}
	if w.renamed != nil {
		w.renamed()
	}
	return nil
}

func (w *writeFS) Remove(name string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.files, mapName(name))
	return nil
}

func (w *writeFS) MkdirAll(name string) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if name = mapName(name); name != "" && path.Clean(name) != "." {
		w.files[name] = &fstest.MapFile{Mode: fs.ModeDir}
	}
	return nil
}
