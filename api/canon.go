package canon

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"sync"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
)

// Options configures a Project; the zero value is valid (API.md §2.1).
type Options struct {
	Layers      []string
	Lang        string
	EditLayer   string
	Roots       map[string]string
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
	root   string
	b      *build.Project
	mu     sync.Mutex
	rev    Revision
	closed bool
}

// FindProject returns the directory holding project.canon in dir or a parent (rule O1).
func FindProject(dir string) (root string, err error) {
	defer recoverInternal(&err)
	abs, err := absolute(dir)
	if err != nil {
		return "", err
	}
	set := &source.FileSet{}
	bag := diag.NewBag(set, "")
	root, err = project.Find(project.OS(), abs, bag)
	switch {
	case errors.Is(err, project.ErrNoProject):
		return "", &ProjectError{Err: ErrNoProject, Findings: fromDiag(set, bag.Findings())}
	case err != nil:
		return "", fmt.Errorf(fmtWrap, ErrNoProject, err)
	}
	return root, nil
}

// Open opens the project whose project.canon is in root (rules O2-O5).
func Open(root string, opts Options) (p *Project, err error) {
	defer recoverInternal(&err)
	dir, err := absolute(root)
	if err != nil {
		return nil, err
	}
	fsys := project.OS()
	if opts.FS != nil {
		fsys = opts.FS
	}
	b, err := build.Open(fsys, dir, build.Options{Roots: opts.Roots, Layers: opts.Layers, MaxFindings: opts.MaxFindings})
	if err != nil {
		return nil, apiError(err)
	}
	return &Project{root: dir, b: b}, nil
}

// Close releases the project and stops every Watch (rule O6).
func (p *Project) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closed = true
	return nil
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
func (p *Project) Packages(ctx context.Context) (infos []PackageInfo, err error) {
	defer recoverInternal(&err)
	b, err := p.open()
	if err != nil {
		return nil, err
	}
	units, err := b.Packages(ctx)
	if err != nil {
		return nil, apiError(err)
	}
	p.setRevision(units.Revision)
	for _, u := range units.Units {
		info := PackageInfo{Name: u.Name, Dir: u.Dir, Imports: u.Imports, Layers: u.Layers}
		for _, f := range u.Files {
			info.Files = append(info.Files, f.Src.Path)
		}
		infos = append(infos, info)
	}
	return infos, nil
}

// Revision identifies a snapshot of the files a project has read (rules S3-S6).
type Revision string

// Revision returns the revision of the current snapshot, after a refresh (rule S1), a broken
// project.canon included; after Close, or a panic, the last revision read.
func (p *Project) Revision() Revision {
	if b, err := p.open(); err == nil {
		p.refresh(b)
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.rev
}

// refresh reads the revision of the current snapshot (rule S3); a panic keeps the last one,
// since Revision has no error to carry it (rule X2).
func (p *Project) refresh(b *build.Project) {
	defer func() { _ = recover() }()
	if rev, err := b.Revision(context.Background()); err == nil {
		p.setRevision(rev)
	}
}

// SetOverlay replaces a file's content in memory without writing it (API.md §3.4).
func (p *Project) SetOverlay(file string, content []byte) error {
	return errUnimplemented()
}

// ClearOverlay removes the overlay of file, if any.
func (p *Project) ClearOverlay(file string) error {
	return errUnimplemented()
}
