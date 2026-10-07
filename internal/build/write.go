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
	remove       bool // delete the file: renamed aside, then removed once every other write is done
}

// commit writes what changed, or, in Check mode, marks it stale and writes nothing (CLI.md §3.4).
func (r *run) commit(check bool, out *BuildResult, outputs []*output, locks []*lockOut) error {
	var changes, removals []change
	for _, o := range outputs {
		switch {
		case o.remove:
			removals = append(removals, change{abs: o.Abs, display: o.Path, remove: true})
		case o.Status == StatusWritten || o.Status == StatusAdopted:
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
	changes = append(changes, removals...)
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
	return writeAll(fsys, changes, r.bounds().allows)
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

// ModeFS is a file system that sets a file's permissions; a replaced file keeps its own.
type ModeFS interface {
	Chmod(name string, mode fs.FileMode) error
}

// writer is one all-or-nothing write: the directories it created, removed again on a failure,
// and the directories it may create (nil: any).
type writer struct {
	fsys    WriteFS
	created []string
	may     func(dir string) bool
}

// writeAll writes every file to a temporary file beside it, then renames each over its target,
// creating only the directories may allows; after a failure, the files already renamed get
// their previous content back and the directories it created are removed (API.md N11).
func writeAll(fsys WriteFS, changes []change, may func(dir string) bool) error {
	w := &writer{fsys: fsys, may: may}
	for i, c := range changes {
		if err := w.stage(c); err != nil {
			removeTemps(fsys, changes[:i+1])
			w.removeDirs()
			return displayError(c.display, err)
		}
	}
	for i, c := range changes {
		if err := replace(fsys, c); err != nil {
			removeTemps(fsys, changes[i:])
			err = errors.Join(displayError(c.display, err), restore(fsys, changes[:i]))
			w.removeDirs()
			return err
		}
	}
	return deleteAside(fsys, removals(changes))
}

// deleteAside deletes the files set aside by a write whose renames all succeeded; a failure says so, since the outputs are written, and the next build sweeps what is left (CODEGEN.md §2.9).
func deleteAside(fsys WriteFS, changes []change) error {
	var errs []error
	for _, c := range changes {
		if err := fsys.Remove(tempOf(c.abs)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			errs = append(errs, fmt.Errorf(fmtAsideLeft, c.display, err))
		}
	}
	return errors.Join(errs...)
}

// replace puts a change's staged file in its place, or sets the file aside under its temporary name when the change deletes it (CODEGEN.md §2.9).
func replace(fsys WriteFS, c change) error {
	if c.remove {
		return fsys.Rename(c.abs, tempOf(c.abs))
	}
	return fsys.Rename(tempOf(c.abs), c.abs)
}

// removals are the changes that delete a file.
func removals(changes []change) []change {
	return slices.DeleteFunc(slices.Clone(changes), func(c change) bool { return !c.remove })
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
	if c.remove {
		return nil
	}
	if err := w.mkdirs(project.DirOf(c.abs)); err != nil {
		return err
	}
	tmp := tempOf(c.abs)
	if err := writeTemp(w.fsys, tmp, c.data); err != nil {
		return err
	}
	if !c.existed {
		return nil
	}
	return keepMode(w.fsys, tmp, c.abs)
}

// keepMode gives tmp the permissions of the file abs, where the file system sets them (handoff 2026-10-07 A2).
func keepMode(fsys WriteFS, tmp, abs string) error {
	chmod, ok := fsys.(ModeFS)
	if !ok {
		return nil
	}
	info, err := fsys.Stat(abs)
	if err != nil {
		return fmt.Errorf(fmtWrap, err)
	}
	return chmod.Chmod(tmp, info.Mode().Perm())
}

// mkdirs creates dir and records the directories that did not exist; one it may not create fails (CODEGEN.md §2.4).
func (w *writer) mkdirs(dir string) error {
	var missing []string
	for d := dir; d != project.DirOf(d); d = project.DirOf(d) {
		if _, err := w.fsys.Stat(d); !errors.Is(err, fs.ErrNotExist) {
			break
		}
		if w.may != nil && !w.may(d) {
			return &fs.PathError{Op: opMkdir, Path: d, Err: errRootMissing}
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
	return project.Join(project.DirOf(abs), tempPrefix+path.Base(abs)+tempSuffix)
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
	if c.remove {
		return fsys.Rename(tempOf(c.abs), c.abs)
	}
	if !c.existed {
		return fsys.Remove(c.abs)
	}
	if err := writeTemp(fsys, tempOf(c.abs), c.old); err != nil {
		return err
	}
	if err := keepMode(fsys, tempOf(c.abs), c.abs); err != nil {
		return err
	}
	return fsys.Rename(tempOf(c.abs), c.abs)
}

// writeTemp removes whatever sits at a fixed temporary name, never writing through it (ADR-0010).
func writeTemp(fsys WriteFS, tmp string, data []byte) error {
	if err := fsys.Remove(tmp); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return fsys.WriteFile(tmp, data)
}
