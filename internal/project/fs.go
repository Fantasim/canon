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
