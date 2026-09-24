package build

import (
	"cmp"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"path"
	"slices"

	"github.com/fantasim/canonlang/internal/check"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// Options configure a project (API.md §2.1).
type Options struct {
	Roots       map[string]string // --root overrides: name to directory, relative to the project's
	Layers      []string
	MaxFindings int // findings kept per package; 0 keeps diag.DefaultMaxFindings (API.md F7)
	Checker     Checker
}

// Checker replaces phase 2, check.Check with eval's folder, over the files of the loaded
// packages and their bags; a test's seam. A nil program it returns is an internal error.
type Checker func(ctx context.Context, proj *project.Project, files []*syntax.File, bags map[string]*diag.Bag) *check.Program

// Project is an opened project: its directory, file system and options (API.md O2). Every call
// reads project.canon and the file set anew (API.md S1).
type Project struct {
	fs  project.FS
	dir string
	opt Options
}

// Findings are findings with the files that locate them, and their counts (API.md §4.3).
type Findings struct {
	Files   diag.Files
	List    []diag.Finding
	Summary diag.Summary
}

// OpenError is a call stopped by project.canon or a root override (API.md O3); Err is
// project.ErrNoProject, project.ErrInvalid or project.ErrUnsupportedVersion.
type OpenError struct {
	Err      error
	Findings Findings
}

func (e *OpenError) Error() string { return e.Err.Error() }

func (e *OpenError) Unwrap() error { return e.Err }

// Open checks project.canon in dir (absolute, '/'-separated), places the roots and scans the
// file set; it parses no other file (API.md O2, O3).
func Open(fsys project.FS, dir string, opt Options) (*Project, error) {
	p := &Project{fs: fsys, dir: path.Clean(dir), opt: opt}
	if _, err := p.open(); err != nil {
		return nil, err
	}
	return p, nil
}

// open starts a snapshot: project.canon read and checked into the project's own bag (package
// "", no package, never truncated), the roots placed, the file set scanned.
func (p *Project) open() (*snapshot, error) {
	set := &source.FileSet{}
	s := &snapshot{p: p, set: set, own: diag.NewBag(set, ""), bags: map[string]*diag.Bag{}}
	if err := p.readProject(s); err != nil {
		var oe *OpenError
		if errors.As(err, &oe) {
			oe.Findings = collect(set, s.own)
		}
		return nil, err
	}
	names, err := project.Scan(p.fs, p.dir)
	if err != nil {
		return nil, displayErrorIn(p.dir, err)
	}
	s.names = names
	return s, nil
}

// readProject reads project.canon into s; a finding that stops the call is an *OpenError.
func (p *Project) readProject(s *snapshot) error {
	if err := project.Require(p.fs, p.dir, s.own); err != nil {
		if errors.Is(err, project.ErrNoProject) {
			return &OpenError{Err: err}
		}
		return displayErrorIn(p.dir, err)
	}
	file := path.Join(p.dir, project.FileName)
	content, err := p.fs.ReadFile(file)
	if err != nil {
		return displayError(project.FileName, err)
	}
	s.sums = append(s.sums, project.FileSum{Path: project.FileName, Sum: sha256.Sum256(content)})
	src, err := s.set.Add(project.FileName, file, content)
	if err != nil {
		return fmt.Errorf(fmtWrap, err)
	}
	if s.proj, err = project.Load(src, s.own); err != nil {
		return &OpenError{Err: err}
	}
	layout, ok := project.NewLayout(s.proj, p.dir, p.opt.Roots, s.own)
	if !ok {
		return &OpenError{Err: project.ErrInvalid}
	}
	s.layout = layout
	return nil
}

func (p *Project) newBag(set *source.FileSet, pkg string) *diag.Bag {
	bag := diag.NewBag(set, pkg)
	if p.opt.MaxFindings > 0 {
		bag.Truncate(p.opt.MaxFindings)
	}
	return bag
}

// collect is the findings of the project's own bag (package "", no package in the counts) and
// of the bags of packages.
func collect(files diag.Files, own *diag.Bag, pkgs ...*diag.Bag) Findings {
	s := own.Summary()
	s.Packages = 0
	out := Findings{Files: files, List: own.Findings(), Summary: s}
	for _, b := range pkgs {
		out.List = append(out.List, b.Findings()...)
		out.Summary = out.Summary.Merge(b.Summary())
	}
	return out
}

// addErrors adds the error findings of an imported package, and their counts, the package
// itself not counted as checked (DECISIONS 196).
func (f *Findings) addErrors(b *diag.Bag) {
	sum := b.Summary()
	if sum.Errors == 0 {
		return
	}
	for _, x := range b.Findings() {
		if x.Severity == diag.Error {
			f.List = append(f.List, x)
		}
	}
	f.Summary.Errors += sum.Errors
	for _, t := range sum.Truncated {
		if t.Errors > 0 {
			f.Summary.Truncated = append(f.Summary.Truncated, diag.Truncation{Package: t.Package, Errors: t.Errors})
		}
	}
	slices.SortFunc(f.Summary.Truncated, func(a, b diag.Truncation) int { return cmp.Compare(a.Package, b.Package) })
}
