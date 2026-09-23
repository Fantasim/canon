package canon

import (
	"context"
	"io/fs"
	"log/slog"
)

// Options configures a Project; the zero value is valid (API.md §2.1).
type Options struct {
	Layers      []string
	Lang        string
	EditLayer   string
	Roots       map[string]string // a relative directory is relative to the project root
	FS          FS
	Cache       string
	Workers     int
	MaxFindings int
	Logger      *slog.Logger
}

// FS is the file system of a Project; WriteFile must be atomic (API.md §2.2).
type FS interface {
	ReadFile(name string) ([]byte, error)
	Stat(name string) (fs.FileInfo, error)
	ReadDir(name string) ([]fs.DirEntry, error)
	WriteFile(name string, data []byte) error
	Rename(oldname, newname string) error
	Remove(name string) error
	MkdirAll(name string) error
}

// Project is an opened Canon project, safe for concurrent use (API.md §3).
type Project struct {
	root string
}

// FindProject returns the directory holding project.canon in dir or a parent (rule O1).
func FindProject(dir string) (root string, err error) {
	return "", errUnimplemented()
}

// Open opens the project whose project.canon is in root (rules O2-O5).
func Open(root string, opts Options) (*Project, error) {
	return nil, errUnimplemented()
}

// Close releases the project and stops every Watch (rule O6).
func (p *Project) Close() error {
	return errUnimplemented()
}

// Root returns the absolute project root directory.
func (p *Project) Root() string {
	return p.root
}

// PackageInfo describes one package of the project (API.md §5.5).
type PackageInfo struct {
	Name    string
	Dir     string
	Files   []string
	Imports []string
	Layers  []string
}

// Packages lists every package of the project, sorted by name.
func (p *Project) Packages(ctx context.Context) ([]PackageInfo, error) {
	return nil, errUnimplemented()
}

// Revision identifies a snapshot of the files a project has read (rules S3-S6).
type Revision string

// Revision returns the revision of the current snapshot, after a refresh (rule S1).
func (p *Project) Revision() Revision {
	panic(msgUnimplemented)
}

// SetOverlay replaces a file's content in memory without writing it (API.md §3.4).
func (p *Project) SetOverlay(file string, content []byte) error {
	return errUnimplemented()
}

// ClearOverlay removes the overlay of file, if any.
func (p *Project) ClearOverlay(file string) error {
	return errUnimplemented()
}
