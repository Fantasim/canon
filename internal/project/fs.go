package project

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
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

// summer is the optional FS capability of a file system that fixes each file's content while it
// is read, as a workspace snapshot does (API.md S1): it knows the SHA-256 of a file without
// handing out a copy of it (log-2026-09-29 P18).
type summer interface {
	SumFile(name string) (sha256Sum, bool, error)
}

// SumFile is the SHA-256 of what fsys.ReadFile(name) gives and that read's error, when fsys has a
// method SumFile(name string) ([32]byte, bool, error) that knows it; false: read the file instead.
func SumFile(fsys FS, name string) (sha256Sum, bool, error) {
	s, ok := fsys.(summer)
	if !ok {
		return sha256Sum{}, false, nil
	}
	return s.SumFile(name)
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

// EvalSymlinks resolves name itself, project's own walk (evalSymlinksOS), never the OS's own EvalSymlinks or its ELOOP wording.
func (osFS) EvalSymlinks(name string) (string, error) {
	return evalSymlinksOS(name)
}
