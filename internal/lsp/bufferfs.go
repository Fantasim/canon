package lsp

import (
	"bytes"
	"fmt"
	"io/fs"
	"path"
	"sync"
	"time"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/project"
)

// bufferFS is the read-only file system a project opens over: the open buffers before the disk
// until release, then the disk alone, under the overlays.
type bufferFS struct {
	disk    build.WriteFS
	mu      sync.Mutex
	buffers map[string][]byte
}

func newBufferFS(disk build.WriteFS, buffers map[string][]byte) *bufferFS {
	return &bufferFS{disk: disk, buffers: buffers}
}

// release stops serving the buffers: from then on the overlays do.
func (f *bufferFS) release() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.buffers = nil
}

func (f *bufferFS) buffer(name string) ([]byte, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	data, ok := f.buffers[name]
	return data, ok
}

func (f *bufferFS) ReadFile(name string) ([]byte, error) {
	if data, ok := f.buffer(name); ok {
		return bytes.Clone(data), nil
	}
	return wrapFS(f.disk.ReadFile(name))
}

func (f *bufferFS) Stat(name string) (fs.FileInfo, error) {
	if data, ok := f.buffer(name); ok {
		return bufferInfo{name: path.Base(name), size: int64(len(data))}, nil
	}
	return wrapFS(f.disk.Stat(name))
}

func (f *bufferFS) ReadDir(name string) ([]fs.DirEntry, error) {
	return wrapFS(f.disk.ReadDir(name))
}

// EvalSymlinks keeps the disk's links followed (WIRE.md §6.5).
func (f *bufferFS) EvalSymlinks(name string) (string, error) {
	return wrapFS(project.EvalSymlinks(f.disk, name))
}

// Readlink keeps the disk's link texts readable (build.LinkReader).
func (f *bufferFS) Readlink(name string) (string, error) {
	r, ok := f.disk.(build.LinkReader)
	if !ok {
		return "", fs.ErrInvalid
	}
	return wrapFS(r.Readlink(name))
}

// wrapFS is a disk result, its error wrapped.
func wrapFS[T any](v T, err error) (T, error) {
	if err != nil {
		return v, fmt.Errorf(fmtWrap, errDisk, err)
	}
	return v, nil
}

// bufferInfo is the stat of a buffer: a regular file of its size.
type bufferInfo struct {
	name string
	size int64
}

func (b bufferInfo) Name() string       { return b.name }
func (b bufferInfo) Size() int64        { return b.size }
func (b bufferInfo) Mode() fs.FileMode  { return 0 }
func (b bufferInfo) ModTime() time.Time { return time.Time{} }
func (b bufferInfo) IsDir() bool        { return false }
func (b bufferInfo) Sys() any           { return nil }
