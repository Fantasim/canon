package cli

import (
	"io/fs"
	"testing"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

const shareRoot = "//server/share/p"

// hostVolumes skips a test on a host whose names have no volume: there "//server/share" is a path.
func hostVolumes(t *testing.T) {
	t.Helper()
	if project.HostPaths().Volume(shareRoot) == "" {
		t.Skip("the host's names carry no volume")
	}
}

// diskOf serves the files it holds by their exact names, and reads nothing else.
type diskOf struct {
	build.WriteFS
	files map[string]string
}

func (d diskOf) ReadFile(name string) ([]byte, error) {
	data, ok := d.files[name]
	if !ok {
		return nil, &fs.PathError{Op: "read", Path: name, Err: fs.ErrNotExist}
	}
	return []byte(data), nil
}

// CLI.md §3.6, API.md §2.2: fmt finds project.canon beside a project on a UNC share, keeping the volume of the name it reads.
func TestFmtProjectBrokenOnUNCVolume(t *testing.T) {
	hostVolumes(t)
	r := &fmtRun{root: shareRoot, fsys: diskOf{files: map[string]string{shareRoot + "/project.canon": "project {\n"}}}
	if !r.projectBroken() {
		t.Error("a project.canon with a syntax error, on the share, was not found")
	}
}

// CLI.md §3.5, API.md §2.2: explain quotes a source of a project on a UNC share by the name the compiler read it under.
func TestExplainSourceOnUNCVolume(t *testing.T) {
	hostVolumes(t)
	const text = "/// A.\npackage a\n\nconst N = 1\n"
	fsys := &sourceFS{WriteFS: diskOf{}, read: map[string][]byte{shareRoot + "/a/a.canon": []byte(text)}}
	s := &sources{fs: fsys, root: shareRoot, set: source.FileSet{}, files: map[string]*syntax.File{}}
	if s.file("a/a.canon") == nil {
		t.Error("a source of a project on the share was not found")
	}
}
