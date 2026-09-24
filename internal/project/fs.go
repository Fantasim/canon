package project

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// FS reads the project's files; names are absolute and '/'-separated (API.md §2.2).
type FS interface {
	ReadFile(name string) ([]byte, error)
	Stat(name string) (fs.FileInfo, error)
	ReadDir(name string) ([]fs.DirEntry, error)
}

// linkResolver is the optional FS capability load.dir follows links through (WIRE.md §6.5).
type linkResolver interface {
	EvalSymlinks(name string) (string, error)
}

// EvalSymlinks is name's real path, every symbolic link in it followed, absolute and
// '/'-separated, when fsys has a method EvalSymlinks(name string) (string, error); an FS
// without one resolves no link (errNoLinks).
func EvalSymlinks(fsys FS, name string) (string, error) {
	r, ok := fsys.(linkResolver)
	if !ok {
		return "", errNoLinks
	}
	return r.EvalSymlinks(name)
}

// OS is the operating system's file system.
func OS() FS { return osFS{} }

type osFS struct{}

func (osFS) ReadFile(name string) ([]byte, error) {
	data, err := os.ReadFile(filepath.FromSlash(name))
	if err != nil {
		return nil, fmt.Errorf(fmtWrap, err)
	}
	return data, nil
}

func (osFS) Stat(name string) (fs.FileInfo, error) {
	info, err := os.Stat(filepath.FromSlash(name))
	if err != nil {
		return nil, fmt.Errorf(fmtWrap, err)
	}
	return info, nil
}

func (osFS) ReadDir(name string) ([]fs.DirEntry, error) {
	entries, err := os.ReadDir(filepath.FromSlash(name))
	if err != nil {
		return nil, fmt.Errorf(fmtWrap, err)
	}
	return entries, nil
}

func (osFS) EvalSymlinks(name string) (string, error) {
	real, err := filepath.EvalSymlinks(filepath.FromSlash(name))
	if err != nil {
		return "", fmt.Errorf(fmtWrap, err)
	}
	return slashed(real, filepath.Separator), nil
}

// slashed is p, an OS path whose separator is osSep, '/'-separated as FS names are (API.md §2.2).
func slashed(p string, osSep rune) string {
	return strings.ReplaceAll(p, string(osSep), sep)
}
