package edit

import (
	"io/fs"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/fantasim/canonlang/internal/project"
)

const (
	shareDir  = "//server/share/p"
	shareFile = shareDir + "/new/x.canon"
)

// shareBase is a share holding only its project directory, its names' "//" kept: a name that
// lost it is not there, as it would not be on the machine.
type shareBase struct{}

func (shareBase) exists(name string) bool { return name == shareDir }

func (b shareBase) Stat(name string) (fs.FileInfo, error) {
	if b.exists(name) {
		return fstest.MapFS{"p": &fstest.MapFile{Mode: fs.ModeDir}}.Stat("p")
	}
	return nil, &fs.PathError{Op: "stat", Path: name, Err: fs.ErrNotExist}
}

func (b shareBase) ReadDir(name string) ([]fs.DirEntry, error) {
	if b.exists(name) {
		return nil, nil
	}
	return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrNotExist}
}

func (shareBase) ReadFile(name string) ([]byte, error) {
	return nil, &fs.PathError{Op: "read", Path: name, Err: fs.ErrNotExist}
}

func (b shareBase) EvalSymlinks(name string) (string, error) {
	if _, err := b.Stat(name); err != nil {
		return "", err
	}
	return name, nil
}

// API.md N8, E18: an edit made in memory on a UNC project directory lists a new file under each directory above it, and resolves it through its directory, the volume kept.
func TestOverlayOnUNCVolume(t *testing.T) {
	if project.HostPaths().Volume(shareDir) == "" {
		t.Skip("the host's names carry no volume")
	}
	o := &overlayFS{base: shareBase{}, files: map[string][]byte{shareFile: []byte("x\n")}}
	for _, c := range []struct {
		dir  string
		want []string
	}{{shareDir, []string{"new"}}, {shareDir + "/new", []string{"x.canon"}}} {
		list, err := o.ReadDir(c.dir)
		var got []string
		for _, e := range list {
			got = append(got, e.Name())
		}
		if err != nil || !slices.Equal(got, c.want) {
			t.Errorf("ReadDir(%q) = %v, %v; want %v", c.dir, got, err, c.want)
		}
	}
	if real, err := o.EvalSymlinks(shareFile); err != nil || real != shareFile {
		t.Errorf("EvalSymlinks(%q) = %q, %v", shareFile, real, err)
	}
}

// API.md §2.2: a volume's root an overlay holds new files under, which its base cannot stat, resolves to itself: EvalSymlinks ends there instead of asking its own parent forever.
func TestOverlayEvalSymlinksVolumeRoot(t *testing.T) {
	o := &overlayFS{base: shareBase{}, files: map[string][]byte{"/new/x.canon": []byte("x\n")}}
	if real, err := o.EvalSymlinks("/"); err != nil || real != "/" {
		t.Errorf("EvalSymlinks(/) = %q, %v", real, err)
	}
	if real, err := o.EvalSymlinks("/new/x.canon"); err != nil || real != "/new/x.canon" {
		t.Errorf("EvalSymlinks(/new/x.canon) = %q, %v", real, err)
	}
}
