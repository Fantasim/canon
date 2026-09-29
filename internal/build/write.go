package build

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/fantasim/canonlang/internal/project"
)

// WriteFS is a project's file system with the writes a build makes (API.md §2.2).
type WriteFS interface {
	project.FS
	WriteFile(name string, data []byte) error
	Rename(oldname, newname string) error
	Remove(name string) error
	MkdirAll(name string) error
}

// OS is the operating system's file system, writable.
func OS() WriteFS { return osFS{FS: project.OS()} }

type osFS struct {
	project.FS
}

func (osFS) WriteFile(name string, data []byte) error {
	return wrapIO(os.WriteFile(filepath.FromSlash(name), data, fileMode))
}

func (osFS) Rename(oldname, newname string) error {
	return wrapIO(os.Rename(filepath.FromSlash(oldname), filepath.FromSlash(newname)))
}

func (osFS) Remove(name string) error { return wrapIO(os.Remove(filepath.FromSlash(name))) }

func (osFS) Chmod(name string, mode fs.FileMode) error {
	return wrapIO(os.Chmod(filepath.FromSlash(name), mode))
}

func (osFS) MkdirAll(name string) error {
	return wrapIO(os.MkdirAll(filepath.FromSlash(name), dirMode))
}

// EvalSymlinks lets load.dir follow links through the OS file system a build reads (WIRE.md §6.5).
func (f osFS) EvalSymlinks(name string) (string, error) { return project.EvalSymlinks(f.FS, name) }

func wrapIO(err error) error {
	if err != nil {
		return fmt.Errorf(fmtWrap, err)
	}
	return nil
}

// lockOut is a lock to write, with the content it replaces (nil: no file).
type lockOut struct {
	Lock
	old []byte
}

// change is one file a build replaces: its new and previous content, and the path a write error names (WIRE.md §2.3).
type change struct {
	abs, display string
	data, old    []byte
	existed      bool
}

// commit writes what changed, or, in Check mode, marks it stale and writes nothing (CLI.md §3.4).
func (r *run) commit(check bool, out *BuildResult, outputs []*output, locks []*lockOut) error {
	var changes []change
	for _, o := range outputs {
		if o.Status == StatusWritten || o.Status == StatusAdopted {
			changes = append(changes, change{abs: o.Abs, display: o.Path, data: o.Content, old: o.old, existed: o.existed})
		}
		out.Outputs = append(out.Outputs, o.Output)
	}
	for _, l := range locks {
		if l.Status == StatusWritten {
			changes = append(changes, change{abs: l.Abs, display: l.Path, data: l.Content, old: l.old, existed: l.old != nil})
		}
		out.Locks = append(out.Locks, l.Lock)
	}
	sortOutputs(out.Outputs)
	if check {
		out.Stale = len(changes) > 0
		markStale(out)
		return nil
	}
	if len(changes) == 0 {
		return nil
	}
	fsys, ok := r.p.fs.(WriteFS)
	if !ok {
		return ErrReadOnly
	}
	return writeAll(fsys, changes)
}

// markStale gives every output and lock a build would write or adopt the status stale, an
// --adopt output included: under --check nothing is written (DECISIONS 201).
func markStale(out *BuildResult) {
	for i := range out.Outputs {
		if out.Outputs[i].Status == StatusWritten || out.Outputs[i].Status == StatusAdopted {
			out.Outputs[i].Status = StatusStale
		}
	}
	for i := range out.Locks {
		if out.Locks[i].Status == StatusWritten {
			out.Locks[i].Status = StatusStale
		}
	}
}

// modeFS is a file system that sets a file's permissions; a replaced file keeps its own.
type modeFS interface {
	Chmod(name string, mode fs.FileMode) error
}

// writer is one all-or-nothing write: the directories it created, removed again on a failure.
type writer struct {
	fsys    WriteFS
	created []string
}

// writeAll writes every file to a temporary file beside it, then renames each over its target;
// after a failure, the files already renamed get their previous content back and the
// directories it created are removed (API.md N11).
func writeAll(fsys WriteFS, changes []change) error {
	w := &writer{fsys: fsys}
	for i, c := range changes {
		if err := w.stage(c); err != nil {
			removeTemps(fsys, changes[:i+1])
			w.removeDirs()
			return displayError(c.display, err)
		}
	}
	for i, c := range changes {
		if err := fsys.Rename(tempOf(c.abs), c.abs); err != nil {
			removeTemps(fsys, changes[i:])
			err = errors.Join(displayError(c.display, err), restore(fsys, changes[:i]))
			w.removeDirs()
			return err
		}
	}
	return nil
}

// WriteAtomic replaces the existing file abs with data through a temporary file beside it, the file's permissions kept; after a failure the file is as it was and no temporary file is left.
func WriteAtomic(fsys WriteFS, abs string, data []byte) error {
	c := change{abs: abs, data: data, existed: true}
	w := &writer{fsys: fsys}
	if err := w.stage(c); err != nil {
		removeTemps(fsys, []change{c})
		w.removeDirs()
		return err
	}
	if err := fsys.Rename(tempOf(abs), abs); err != nil {
		removeTemps(fsys, []change{c})
		return err
	}
	return nil
}

// displayErr prints a display path, op and cause in place of an OS error's absolute path, and
// unwraps to the whole original error, so errors.Is and errors.As see all it wrapped.
type displayErr struct {
	msg string
	err error
}

// Error is the display message.
func (e *displayErr) Error() string { return e.msg }

// Unwrap is the original error.
func (e *displayErr) Unwrap() error { return e.err }

// displayError names display in place of the absolute path (or two, a rename's) a *fs.PathError
// or *os.LinkError carries, its op and cause kept (DECISIONS 201).
func displayError(display string, err error) error {
	if pe, ok := asPathError(err); ok {
		return &displayErr{msg: fmt.Sprintf(fmtDisplayOpCause, display, pe.Op, pe.Err), err: err}
	}
	return fmt.Errorf(fmtDisplayCause, display, err)
}

// asPathError is err's *fs.PathError, or its rename's *os.LinkError as one naming the target;
// their paths are in the OS's form, usually absolute, and never printed as they are.
func asPathError(err error) (*fs.PathError, bool) {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return pe, true
	}
	var le *os.LinkError
	if errors.As(err, &le) {
		return &fs.PathError{Op: le.Op, Path: le.New, Err: le.Err}, true
	}
	return nil, false
}

// displayErrorIn names err's path by its display path in the project at dir (DECISIONS 201).
func displayErrorIn(dir string, err error) error {
	pe, ok := asPathError(err)
	if !ok {
		return err
	}
	return displayError(relativeTo(dir, pe.Path, filepath.Separator), err)
}

// relativeTo is the '/'-separated display path of name, an OS path separated by sep, in dir:
// "." for dir, dir's prefix cut at a separator, and only the last element of a path outside it.
func relativeTo(dir, name string, sep rune) string {
	abs := path.Clean(strings.ReplaceAll(name, string(sep), pathSep))
	root := path.Clean(dir)
	if abs == root {
		return listingMark
	}
	if rel, ok := strings.CutPrefix(abs, strings.TrimSuffix(root, pathSep)+pathSep); ok {
		return rel
	}
	if base := path.Base(abs); base != pathSep {
		return base
	}
	return listingMark
}

// stage writes a change's temporary file, its directory created, an existing file's mode kept.
func (w *writer) stage(c change) error {
	if err := w.mkdirs(path.Dir(c.abs)); err != nil {
		return err
	}
	tmp := tempOf(c.abs)
	if err := w.fsys.WriteFile(tmp, c.data); err != nil {
		return err
	}
	chmod, ok := w.fsys.(modeFS)
	if !c.existed || !ok {
		return nil
	}
	info, err := w.fsys.Stat(c.abs)
	if err != nil {
		return fmt.Errorf(fmtWrap, err)
	}
	return chmod.Chmod(tmp, info.Mode().Perm())
}

// mkdirs creates dir and records the directories that did not exist.
func (w *writer) mkdirs(dir string) error {
	var missing []string
	for d := dir; d != path.Dir(d); d = path.Dir(d) {
		if _, err := w.fsys.Stat(d); !errors.Is(err, fs.ErrNotExist) {
			break
		}
		missing = append(missing, d)
	}
	w.created = append(w.created, missing...)
	return w.fsys.MkdirAll(dir)
}

// removeDirs removes the directories the write created, deepest first.
func (w *writer) removeDirs() {
	slices.SortFunc(w.created, func(a, b string) int { return cmp.Or(cmp.Compare(len(b), len(a)), cmp.Compare(a, b)) })
	for _, d := range slices.Compact(w.created) {
		_ = w.fsys.Remove(d) // one another writer filled meanwhile stays
	}
}

// tempOf is the temporary file a change is written to first: hidden, in the target's directory.
func tempOf(abs string) string {
	return path.Join(path.Dir(abs), tempPrefix+path.Base(abs)+tempSuffix)
}

func removeTemps(fsys WriteFS, changes []change) {
	for _, c := range changes {
		_ = fsys.Remove(tempOf(c.abs)) // a temporary file that was never written is no failure
	}
}

// restore puts back the previous content of files already replaced, or removes a new one; each
// failure names its display path, not the temporary one (DECISIONS 201).
func restore(fsys WriteFS, done []change) error {
	var errs []error
	for _, c := range done {
		if err := restoreOne(fsys, c); err != nil {
			errs = append(errs, displayError(c.display, err))
		}
	}
	return errors.Join(errs...)
}

// restoreOne restores one file already replaced, or removes one that did not exist before.
func restoreOne(fsys WriteFS, c change) error {
	if !c.existed {
		return fsys.Remove(c.abs)
	}
	if err := fsys.WriteFile(tempOf(c.abs), c.old); err != nil {
		return err
	}
	return fsys.Rename(tempOf(c.abs), c.abs)
}
