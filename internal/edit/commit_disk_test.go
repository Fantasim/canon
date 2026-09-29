package edit_test

import (
	"io/fs"
	"maps"
	"path"
	"slices"
	"strings"
	"sync"
	"time"
)

// diskFS is a writable FS in memory with an atomic WriteFile and symbolic links (API.md §2.2).
type diskFS struct {
	mu    sync.Mutex
	files map[string][]byte
	modes map[string]fs.FileMode // files' and directories'
	dirs  map[string]bool
	links map[string]string // a link's name to the absolute name it leads to
}

func newDiskFS(files map[string]string) *diskFS {
	d := &diskFS{
		files: map[string][]byte{}, modes: map[string]fs.FileMode{"/": 0o755},
		dirs: map[string]bool{"/": true}, links: map[string]string{},
	}
	for _, name := range slices.Sorted(maps.Keys(files)) {
		d.mkdirs(path.Dir(name))
		d.files[name] = []byte(files[name])
		d.modes[name] = 0o644
	}
	return d
}

func (d *diskFS) mkdirs(name string) {
	for p := name; !d.dirs[p]; p = path.Dir(p) {
		d.dirs[p] = true
		d.modes[p] = 0o755
	}
}

// link makes name a symbolic link to target.
func (d *diskFS) link(name, target string) {
	d.mkdirs(path.Dir(name))
	d.links[name] = target
}

// follow is name with every link along it followed.
func (d *diskFS) follow(name string) string {
	out := "/"
	for _, seg := range strings.Split(strings.TrimPrefix(name, "/"), "/") {
		out = path.Join(out, seg)
		if target, ok := d.links[out]; ok {
			out = target
		}
	}
	return out
}

// followDir is name with the links of its directory followed, not its own: what Remove and
// Rename act on.
func (d *diskFS) followDir(name string) string {
	return path.Join(d.follow(path.Dir(name)), path.Base(name))
}

// children are the files, directories and links right in dir, in byte order.
func (d *diskFS) children(dir string) []string {
	var out []string
	all := slices.Concat(slices.Collect(maps.Keys(d.dirs)), slices.Collect(maps.Keys(d.files)), slices.Collect(maps.Keys(d.links)))
	for _, name := range all {
		if name != dir && path.Dir(name) == dir {
			out = append(out, name)
		}
	}
	slices.Sort(out)
	return out
}

// state is every file, directory but .canon's and link, with modes and contents, as one
// comparable text: a commit may leave the journal directory, never a file in it.
func (d *diskFS) state() string { return d.stateBut(func(string) bool { return false }) }

// settled is the state but journals and stages: what a rollback has to put back.
func (d *diskFS) settled() string {
	return d.stateBut(func(name string) bool {
		return strings.Contains(name, "/.canon/journal/") || strings.HasSuffix(name, ".canon-edit")
	})
}

func (d *diskFS) stateBut(skip func(string) bool) string {
	d.mu.Lock()
	defer d.mu.Unlock()
	var b strings.Builder
	for _, name := range slices.Sorted(maps.Keys(d.files)) {
		if !skip(name) {
			b.WriteString(name + " " + d.modes[name].String() + " " + string(d.files[name]) + "\x00\n")
		}
	}
	for _, name := range slices.Sorted(maps.Keys(d.dirs)) {
		if !strings.Contains(name, "/.canon") {
			b.WriteString(name + "/ " + d.modes[name].String() + "\n")
		}
	}
	for _, name := range slices.Sorted(maps.Keys(d.links)) {
		b.WriteString(name + " -> " + d.links[name] + "\n")
	}
	return b.String()
}

func notExist(op, name string) error { return &fs.PathError{Op: op, Path: name, Err: fs.ErrNotExist} }

func (d *diskFS) ReadFile(name string) ([]byte, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	data, ok := d.files[d.follow(name)]
	if !ok {
		return nil, notExist("read", name)
	}
	return slices.Clone(data), nil
}

type info struct {
	name string
	size int64
	mode fs.FileMode
}

func (i info) Name() string       { return i.name }
func (i info) Size() int64        { return i.size }
func (i info) Mode() fs.FileMode  { return i.mode }
func (i info) ModTime() time.Time { return time.Time{} }
func (i info) IsDir() bool        { return i.mode.IsDir() }
func (i info) Sys() any           { return nil }

// stat is the entry at a name already followed, a link itself included.
func (d *diskFS) stat(name string) (fs.FileInfo, bool) {
	if data, ok := d.files[name]; ok {
		return info{name: path.Base(name), size: int64(len(data)), mode: d.modes[name]}, true
	}
	if d.dirs[name] {
		return info{name: path.Base(name), mode: fs.ModeDir | d.modes[name]}, true
	}
	if _, ok := d.links[name]; ok {
		return info{name: path.Base(name), mode: fs.ModeSymlink | 0o777}, true
	}
	return nil, false
}

func (d *diskFS) Stat(name string) (fs.FileInfo, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if i, ok := d.stat(d.follow(name)); ok {
		return i, nil
	}
	return nil, notExist("stat", name)
}

func (d *diskFS) ReadDir(name string) ([]fs.DirEntry, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	dir := d.follow(name)
	if !d.dirs[dir] {
		return nil, notExist("readdir", name)
	}
	var out []fs.DirEntry
	for _, child := range d.children(dir) {
		i, _ := d.stat(child)
		out = append(out, fs.FileInfoToDirEntry(i))
	}
	return out, nil
}

// EvalSymlinks is name's real path, as the OS FS's (WIRE.md §6.5): absent when nothing is there.
func (d *diskFS) EvalSymlinks(name string) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	real := d.follow(name)
	if _, ok := d.stat(real); !ok {
		return "", notExist("lstat", name)
	}
	return real, nil
}

func (d *diskFS) WriteFile(name string, data []byte) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	real := d.follow(name)
	if !d.dirs[path.Dir(real)] {
		return notExist("open", name)
	}
	d.files[real] = slices.Clone(data)
	if _, ok := d.modes[real]; !ok {
		d.modes[real] = 0o600
	}
	return nil
}

func (d *diskFS) Rename(oldname, newname string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	from, to := d.followDir(oldname), d.followDir(newname)
	data, ok := d.files[from]
	if !ok || !d.dirs[path.Dir(to)] {
		return notExist("rename", oldname)
	}
	delete(d.links, to)
	d.files[to], d.modes[to] = data, d.modes[from]
	delete(d.files, from)
	delete(d.modes, from)
	return nil
}

func (d *diskFS) Remove(name string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	real := d.followDir(name)
	if _, ok := d.links[real]; ok {
		delete(d.links, real)
		return nil
	}
	if _, ok := d.files[real]; ok {
		delete(d.files, real)
		delete(d.modes, real)
		return nil
	}
	if !d.dirs[real] {
		return notExist("remove", name)
	}
	if len(d.children(real)) > 0 {
		return errNotEmpty
	}
	delete(d.dirs, real)
	delete(d.modes, real)
	return nil
}

func (d *diskFS) MkdirAll(name string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.mkdirs(d.follow(name))
	return nil
}

func (d *diskFS) Chmod(name string, mode fs.FileMode) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	real := d.follow(name)
	if _, ok := d.stat(real); !ok {
		return notExist("chmod", name)
	}
	d.modes[real] = mode
	return nil
}
