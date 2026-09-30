package cli

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"path"
	"slices"
	"time"

	canon "github.com/fantasim/canonlang/api"
	"github.com/fantasim/canonlang/internal/build"
	"github.com/fantasim/canonlang/internal/diag"
	"github.com/fantasim/canonlang/internal/format"
	"github.com/fantasim/canonlang/internal/load"
	"github.com/fantasim/canonlang/internal/project"
	"github.com/fantasim/canonlang/internal/source"
	"github.com/fantasim/canonlang/internal/syntax"
)

// fmtFile is a file canon fmt visits: its project-relative path and what it parses as.
type fmtFile struct {
	display string
	kind    syntax.FileKind
}

// fmtChange is a file whose canonical layout differs from its text: abs is the file itself, a link resolved.
type fmtChange struct {
	display, abs string
	old, updated []byte
}

// fmtRun is one canon fmt: the files it read, what it found and what it would change.
type fmtRun struct {
	inv      *invocation
	fsys     build.WriteFS
	root     string
	layout   *project.Layout
	set      source.FileSet
	findings []diag.Finding
	failed   bool
	visited  int
	seen     map[string]bool // the resolved paths already visited: a link and its target are one file
	changes  []fmtChange
	sources  []string // the .canon sources that parsed, by display path: --json-sources reads their loads
}

// runFmt is `canon fmt [paths…] [--check] [--diff] [--json-sources]` (CLI.md §3.6).
func runFmt(inv *invocation) int {
	start := time.Now()
	root, err := inv.projectRoot()
	if err != nil {
		return inv.fail(err)
	}
	r := &fmtRun{inv: inv, fsys: build.OS(), root: project.HostPaths().FromAPI(root, ""), seen: map[string]bool{}}
	if r.projectBroken() {
		return r.finish(start)
	}
	p, err := inv.openProject()
	if err != nil {
		return inv.fail(err)
	}
	defer func() { _ = p.Close() }()
	r.root = p.Root()
	if err := r.run(p); err != nil {
		return inv.fail(err)
	}
	return r.finish(start)
}

// finish prints the report and is the exit code: 1 for a syntax error, or under --check a file not formatted (CLI.md §2.5).
func (r *fmtRun) finish(start time.Time) int {
	if err := r.report(start); err != nil {
		return r.inv.fail(err)
	}
	if r.failed || r.inv.opt.checkFlag && len(r.changes) > 0 {
		return exitErrors
	}
	return exitOK
}

// projectBroken keeps the findings of a project.canon with a syntax error, exit 1, nothing written (CLI.md §3.6, §2.5).
func (r *fmtRun) projectBroken() bool {
	abs := project.Join(r.root, project.FileName)
	data, err := r.fsys.ReadFile(abs)
	if err != nil {
		return false
	}
	src, err := r.set.Add(project.FileName, abs, data)
	if err != nil {
		return false
	}
	bag := diag.NewBag(&r.set, "")
	if _, err := format.Source(src, syntax.FileProject, bag); !errors.Is(err, format.ErrSyntax) {
		return false
	}
	r.visited++
	r.reject(bag)
	return true
}

// run reads and formats the files, then writes the changes unless --check or --diff asks not to.
func (r *fmtRun) run(p *canon.Project) error {
	if !r.place() {
		return nil
	}
	files, err := r.files(p)
	if err != nil {
		return err
	}
	for _, f := range files {
		if err := r.formatCanon(f); err != nil {
			return err
		}
	}
	if r.inv.opt.jsonSources {
		if err := r.formatJSONSources(); err != nil {
			return err
		}
	}
	slices.SortFunc(r.changes, func(a, b fmtChange) int { return cmp.Compare(a.display, b.display) })
	if r.inv.opt.checkFlag || r.inv.opt.diff {
		return nil
	}
	return r.write()
}

// place lays the roots on disk, --root applied; false after keeping the findings of a project.canon that does not load (WIRE.md §2.2).
func (r *fmtRun) place() bool {
	abs := project.Join(r.root, project.FileName)
	data, err := r.fsys.ReadFile(abs)
	if err != nil {
		r.failed = true
		return false
	}
	src, err := r.set.Add(project.FileName, abs, data)
	if err != nil {
		r.failed = true
		return false
	}
	bag := diag.NewBag(&r.set, "")
	proj, err := project.Load(src, bag)
	if err == nil {
		var ok bool
		if r.layout, ok = project.NewLayout(proj, r.root, project.HostPaths().RootsFromAPI(r.inv.opt.roots, r.root), bag); ok {
			return true
		}
	}
	r.reject(bag)
	return false
}

// files are the files fmt visits, in path order: project.canon and every .canon file, or the selected packages' (CLI.md §2.2).
func (r *fmtRun) files(p *canon.Project) ([]fmtFile, error) {
	if len(r.inv.args) == 0 {
		return r.allFiles()
	}
	pkgs, err := p.Packages(r.inv.ctx)
	if err != nil {
		return nil, err
	}
	listings := make([]project.Listing, len(pkgs))
	for i, pkg := range pkgs {
		listings[i] = project.Listing{Name: pkg.Name, Dir: pkg.Dir, Files: pkg.Files}
	}
	var files []fmtFile
	for i, sel := range r.inv.selectors(r.root) {
		got, err := r.selectFiles(listings, sel)
		if err != nil {
			return nil, typedError(err, r.inv.args[i])
		}
		files = append(files, got...)
	}
	return sortedFiles(files), nil
}

// allFiles are project.canon and every scanned .canon file.
func (r *fmtRun) allFiles() ([]fmtFile, error) {
	names, err := project.Scan(r.fsys, r.root)
	if err != nil {
		return nil, fmt.Errorf(fmtArgs, dotSep, readCause(err))
	}
	files := []fmtFile{{display: project.FileName, kind: syntax.FileProject}}
	for _, name := range names {
		files = append(files, fmtFile{display: name, kind: syntax.FileSource})
	}
	return sortedFiles(files), nil
}

// selectFiles are the files of the packages sel selects; project.canon, and a .canon file that
// declares no package, select themselves.
func (r *fmtRun) selectFiles(listings []project.Listing, sel string) ([]fmtFile, error) {
	if sel == project.FileName {
		return []fmtFile{{display: project.FileName, kind: syntax.FileProject}}, nil
	}
	matched, err := project.Match(listings, sel)
	if err != nil {
		if names, scanErr := project.Scan(r.fsys, r.root); scanErr == nil && slices.Contains(names, path.Clean(sel)) {
			return []fmtFile{{display: path.Clean(sel), kind: syntax.FileSource}}, nil
		}
		return nil, err
	}
	var files []fmtFile
	for _, i := range matched {
		for _, name := range listings[i].Files {
			files = append(files, fmtFile{display: name, kind: syntax.FileSource})
		}
	}
	return files, nil
}

// sortedFiles orders files by path, each once.
func sortedFiles(files []fmtFile) []fmtFile {
	slices.SortFunc(files, func(a, b fmtFile) int { return cmp.Compare(a.display, b.display) })
	return slices.CompactFunc(files, func(a, b fmtFile) bool { return a.display == b.display })
}

// visit is the resolved path of a file fmt may format and has not yet: a link is followed, one leaving the roots is left alone (CLI.md §3.6).
func (r *fmtRun) visit(abs string) (string, bool) {
	real, ok := load.Inside(r.fsys, r.layout, abs)
	if !ok || r.seen[real] || !r.regular(real) {
		return "", false
	}
	r.seen[real] = true
	r.visited++
	return real, true
}

// regular reports a regular file at abs; a missing one is a load `check` reports, not fmt.
func (r *fmtRun) regular(abs string) bool {
	info, err := r.fsys.Stat(abs)
	return err == nil && info.Mode().IsRegular()
}

// formatCanon formats one .canon file: a change to make, or its syntax errors, the file left as it is (FORMATTER.md §16).
func (r *fmtRun) formatCanon(f fmtFile) error {
	if err := r.inv.ctx.Err(); err != nil {
		return err
	}
	abs, ok := r.visit(project.Join(r.root, f.display))
	if !ok {
		return nil
	}
	data, err := r.read(f.display, abs)
	if err != nil {
		return err
	}
	src, err := r.set.Add(f.display, abs, data)
	if err != nil {
		return fmt.Errorf(fmtArgs, f.display, errTooLarge)
	}
	bag := diag.NewBag(&r.set, "")
	out, err := format.Source(src, f.kind, bag)
	switch {
	case errors.Is(err, format.ErrSyntax):
		r.reject(bag)
	case err != nil:
		return fmt.Errorf(fmtWrap, err)
	default:
		if f.kind == syntax.FileSource {
			r.sources = append(r.sources, f.display)
		}
		r.record(f.display, abs, data, out)
	}
	return nil
}

// reject keeps the findings of a file that did not parse; the exit code is 1 even without any.
func (r *fmtRun) reject(bag *diag.Bag) {
	r.findings = append(r.findings, bag.Findings()...)
	r.failed = true
}

// record notes a file whose text is not its canonical layout.
func (r *fmtRun) record(display, abs string, data, updated []byte) {
	if !bytes.Equal(data, updated) {
		r.changes = append(r.changes, fmtChange{display: display, abs: abs, old: data, updated: updated})
	}
}

// read reads a file, its errors named by the display path and a fixed cause, never the OS's text.
func (r *fmtRun) read(display, abs string) ([]byte, error) {
	data, err := r.fsys.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf(fmtArgs, display, readCause(err))
	}
	return data, nil
}

// readCause is the fixed cause of a read that failed, chosen by errors.Is.
func readCause(err error) error {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return errNoFile
	case errors.Is(err, fs.ErrPermission):
		return errDenied
	}
	return errUnreadable
}
