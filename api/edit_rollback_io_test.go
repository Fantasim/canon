package canon_test

import (
	"context"
	"errors"
	"io/fs"
	"path"
	"strings"
	"testing"

	canon "github.com/fantasim/canonlang/api"
)

// denyStagesFS is a memFS whose stage writes fail as the OS WriteFile fails, naming its absolute
// temporary file (DECISIONS 416).
type denyStagesFS struct{ *memFS }

func (d denyStagesFS) WriteFile(name string, data []byte) error {
	if strings.HasSuffix(name, ".canon-edit") {
		return &fs.PathError{Op: "open", Path: name + ".tmp", Err: fs.ErrPermission}
	}
	return d.memFS.WriteFile(name, data)
}

// API.md O5, N11 (DECISIONS 416): after a crash, an Open whose rollback the file system refuses
// fails with ErrProject naming the journal and a/a.canon by display path; the journal is kept.
func TestOpenRollbackIOErrorIsProject(t *testing.T) {
	base := lawFS()
	p, err := canon.Open("/law", canon.Options{FS: &faultFS{memFS: base, failAt: 2, crash: true}})
	if err != nil {
		t.Fatal(err)
	}
	_, _ = p.Edit(context.Background(), threeFiles)
	_ = p.Close()
	names := journals(base)
	if len(names) != 1 {
		t.Fatalf("journals %v", names)
	}
	_, err = canon.Open("/law", canon.Options{FS: denyStagesFS{base}})
	var pe *fs.PathError
	switch {
	case !errors.Is(err, canon.ErrProject) || !errors.Is(err, fs.ErrPermission) || !errors.As(err, &pe):
		t.Fatalf("Open: %v, want ErrProject wrapping the *fs.PathError of fs.ErrPermission", err)
	case !strings.Contains(err.Error(), path.Base(names[0])) || !strings.Contains(err.Error(), "a/a.canon"):
		t.Errorf("Open: %q, want the journal and a/a.canon named", err)
	case strings.Contains(err.Error(), "/law/"):
		t.Errorf("Open: %q names an absolute path", err)
	}
	if len(journals(base)) != 1 {
		t.Error("API.md N11: the journal of a refused rollback was removed")
	}
}
