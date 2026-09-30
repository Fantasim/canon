package load_test

import (
	"io/fs"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/fantasim/canonlang/internal/project"
)

// shareRoot is a project on a network share: its names carry a volume no walk may climb above.
const (
	shareVolume = "//server/share"
	shareRoot   = shareVolume + "/p"
)

// shareFS is a tree under shareVolume as a project.FS; a name that lost the volume's "//" is not
// there, as it would not be on the machine.
type shareFS struct{ m fstest.MapFS }

func newShareFS(files map[string]string) shareFS {
	m := fstest.MapFS{}
	for name, data := range files {
		m[strings.TrimPrefix(shareRoot+"/"+name, "//")] = &fstest.MapFile{Data: []byte(data)}
	}
	return shareFS{m}
}

func (f shareFS) key(name string) (string, error) {
	key, ok := strings.CutPrefix(name, "//")
	if !ok {
		return "", &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	return key, nil
}

func (f shareFS) ReadFile(name string) ([]byte, error) {
	key, err := f.key(name)
	if err != nil {
		return nil, err
	}
	return f.m.ReadFile(key)
}

func (f shareFS) Stat(name string) (fs.FileInfo, error) {
	key, err := f.key(name)
	if err != nil {
		return nil, err
	}
	return f.m.Stat(key)
}

func (f shareFS) ReadDir(name string) ([]fs.DirEntry, error) {
	key, err := f.key(name)
	if err != nil {
		return nil, err
	}
	return f.m.ReadDir(key)
}

var _ project.FS = shareFS{}

// WIRE.md §6.5, API.md §2.2: load.dir walks a project on a UNC share, every name it reads keeping the volume, its display paths project-relative as ever.
func TestLoadDirOnUNCVolume(t *testing.T) {
	if project.HostPaths().Volume(shareRoot) == "" {
		t.Skip("the host's names carry no volume")
	}
	fsys := newShareFS(map[string]string{
		"data/a.json":      `{"id": "a", "name": "A"}`,
		"data/sub/b.json":  `{"id": "b", "name": "B"}`,
		"data/sub/c.txt":   "not json",
		"data/.hid/d.json": `{"id": "d", "name": "D"}`,
	})
	run := runKeyed(t, fsys, layoutAt(t, shareRoot), "data/**/*.json")
	if want := []string{"a", "b"}; !slices.Equal(run.ids, want) {
		t.Errorf("entries = %v, want %v", run.ids, want)
	}
	if want := []string{"data/a.json", "data/sub/b.json"}; !slices.Equal(run.read, want) {
		t.Errorf("files read = %v, want %v", run.read, want)
	}
}
