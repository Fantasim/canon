package canon

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"maps"
	"slices"
	"sync"

	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/edit"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/workspace"
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

// Project is an opened Canon project, safe for concurrent use, over its workspace (API.md §3).
type Project struct {
	root      string
	b         *build.Project
	mu        sync.Mutex
	ws        *workspace.Project // made over b on first use
	rev       Revision           // the revision of the newest snapshot a call read, what Revision returns after Close
	revAt     uint64             // that snapshot's place among those published: rev only advances (S10)
	layers    []string           // Options.Layers, the active layers
	editLayer string             // Options.EditLayer, which Value's Editable is judged with (API.md §7.5)
	lang      string             // Options.Lang, the language of Evaluate's texts unless a request names one (§11)
	logger    *slog.Logger       // Options.Logger, nil to discard
	osFiles   bool               // no Options.FS: a Watch follows the OS's notifications (W12)
	roots     map[string]string  // Options.Roots, which a Watch lays load globs out by (W12)
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
	// The default file system is writable (API.md §2.1 "the OS").
	var fsys project.FS = build.OS()
	if opts.FS != nil {
		fsys = opts.FS
	}
	b, err := build.Open(fsys, dir, build.Options{Roots: opts.Roots, Layers: opts.Layers, MaxFindings: opts.MaxFindings})
	if err != nil {
		return nil, apiError(err)
	}
	if err := workspace.Recover(b, opts.Logger); err != nil {
		return nil, journalError(err)
	}
	return &Project{
		root: dir, b: b, editLayer: opts.EditLayer, layers: slices.Clone(opts.Layers), lang: opts.Lang,
		logger: opts.Logger, osFiles: opts.FS == nil, roots: maps.Clone(opts.Roots),
	}, nil
}

// journalError is a project state that forbids writing, ErrProject with edit's reason, journal
// and files, for the user to resolve (rule O5, log-2026-09-29 M4 U5b-r): an unfinished edit Open
// or a commit met, a path hidden or through a symbolic link; any other error as it is.
func journalError(err error) error {
	if errors.Is(err, edit.ErrJournal) || errors.Is(err, edit.ErrUnwritable) {
		return fmt.Errorf(fmtWrap, ErrProject, err)
	}
	return err
}

// Close releases the project and stops every Watch (rule O6).
func (p *Project) Close() error {
	p.workspace().Close()
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
	s, err := p.read(ctx)
	if err != nil {
		return nil, err
	}
	units, err := share(ctx, s, workspace.Key(workspace.OpPackages, nil), s.Build().Packages)
	if err != nil {
		return nil, err
	}
	if _, err := p.revision(ctx, s); err != nil {
		return nil, err
	}
	for _, u := range units.Units {
		// The units may be shared with another call (S8): the result holds its own slices.
		info := PackageInfo{Name: u.Name, Dir: u.Dir, Imports: slices.Clone(u.Imports), Layers: slices.Clone(u.Layers)}
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
// project.canon included: never one older than a writer had published when it was called (S10);
// after Close, or a panic, the revision of the newest snapshot a call read.
func (p *Project) Revision() Revision {
	if rev, ok := p.refresh(); ok {
		return rev // its own snapshot's: a read ending meanwhile on an older one cannot replace it
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.rev
}

// refresh is the revision of the current snapshot, refreshed (rules S1, S3), false after Close;
// a panic is false too, since Revision has no error to carry it (rule X2).
func (p *Project) refresh() (rev Revision, ok bool) {
	defer func() { _ = recover() }()
	ctx := context.Background()
	s, err := p.read(ctx)
	if err != nil {
		return "", false
	}
	rev, err = p.revision(ctx, s) // no ctx to cancel it: it cannot fail
	return rev, err == nil
}

// SetOverlay replaces a display or absolute path's content in memory, a writer (API.md §3.4).
func (p *Project) SetOverlay(file string, content []byte) (err error) {
	defer recoverInternal(&err)
	return overlayError(file, p.workspace().SetOverlay(file, content))
}

// ClearOverlay removes the overlay of file, if any (API.md §3.4).
func (p *Project) ClearOverlay(file string) (err error) {
	defer recoverInternal(&err)
	return overlayError(file, p.workspace().ClearOverlay(file))
}

// overlayError is an overlay's failure as the API reports it, a bad file ErrBadPath (API.md §15).
func overlayError(file string, err error) error {
	if errors.Is(err, workspace.ErrBadPath) {
		return &PathError{Op: -1, Path: file, Err: ErrBadPath}
	}
	return apiError(err)
}
