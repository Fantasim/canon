//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly || illumos

package workspace

import (
	"context"
	"io/fs"
	"slices"
	"syscall"
	"testing"
	"time"
)

const identPath = "/ident/file.canon"

// identFS is a file system whose one file keeps its size, modification time and change time while
// its identity and content are changed by the test.
type identFS struct {
	content string
	st      syscall.Stat_t
}

func (f *identFS) ReadFile(string) ([]byte, error)       { return []byte(f.content), nil }
func (f *identFS) ReadDir(string) ([]fs.DirEntry, error) { return nil, fs.ErrNotExist }
func (f *identFS) Stat(string) (fs.FileInfo, error)      { return identInfo{f: f}, nil }

type identInfo struct{ f *identFS }

func (i identInfo) Name() string       { return "file.canon" }
func (i identInfo) Size() int64        { return int64(len(i.f.content)) }
func (i identInfo) Mode() fs.FileMode  { return 0 }
func (i identInfo) ModTime() time.Time { return stampTime() }
func (i identInfo) IsDir() bool        { return false }
func (i identInfo) Sys() any           { st := i.f.st; return &st }

// API.md S1, DECISIONS 374: the inode and the device are part of the stamp, so a name whose size,
// modification time and change time are all kept is read again when either moves.
func TestRefreshIdentity(t *testing.T) {
	rows := []struct {
		name  string
		apply func(st *syscall.Stat_t)
	}{
		{"inode", func(st *syscall.Stat_t) { st.Ino++ }},
		{"device", func(st *syscall.Stat_t) { st.Dev++ }},
	}
	for _, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			f := &identFS{content: stampOld}
			f.st.Ino, f.st.Dev = 1, 1
			late := func() time.Time { return time.Now().Add(stampLate) }
			s := newSnapFS(f, nil, late)
			if _, err := s.ReadFile(identPath); err != nil {
				t.Fatal(err)
			}
			f.content = stampNew
			row.apply(&f.st)
			_, changed, err := s.refresh(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if want := []name{{kind: kindFile, abs: identPath}}; !slices.Equal(changed, want) {
				t.Fatalf("changed = %v, want %v", changed, want)
			}
		})
	}
}
